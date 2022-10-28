package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"validation-bot/module/queryask"
	"validation-bot/module/retrieval"
	"validation-bot/module/thousandeyes"
	"validation-bot/role"

	"validation-bot/module"
	echo_module "validation-bot/module/echo"
	"validation-bot/role/dispatcher"

	"validation-bot/role/auditor"
	"validation-bot/role/observer"

	"validation-bot/store"

	"validation-bot/task"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	log2 "github.com/labstack/gommon/log"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log3 "github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
	"github.com/urfave/cli/v2"
	"github.com/ziflex/lecho/v3"
	"gopkg.in/yaml.v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	deleteRoute string = "/task/:id"
	createRoute string = "/task"
	listRoute   string = "/tasks"
)

type taskLister interface {
	List(ctx context.Context) ([]task.Definition, error)
}

type taskCreator interface {
	Create(ctx context.Context, taskDef *task.Definition) error
}

type taskRemover interface {
	Remove(ctx context.Context, id uuid.UUID) error
}

//nolint:gomnd,funlen,cyclop
func setConfig(configPath string) (*config, error) {
	log := log3.With().Str("role", "main").Caller().Logger()
	defaultConnectionString := "host=localhost port=5432 user=postgres password=postgres dbname=postgres"

	cfg := config{
		Log: logConfig{
			Pretty: true,
			Level:  "debug",
		},
		Dispatcher: dispatcherConfig{
			Enabled:                  true,
			DatabaseConnectionString: defaultConnectionString,
			PrivateKey:               "",
			APIAddress:               ":8000",
			ListenAddr:               "/ip4/0.0.0.0/tcp/7998",
			TopicName:                "/filecoin/validation_bot/dev",
			CheckInterval:            time.Minute,
		},
		Auditor: auditorConfig{
			Enabled:      true,
			TrustedPeers: []string{},
			PrivateKey:   "",
			ListenAddr:   "/ip4/0.0.0.0/tcp/7999",
			TopicNames:   []string{"/filecoin/validation_bot/dev"},
			W3S: w3sConfig{
				Token:        "",
				RetryWait:    10 * time.Second,
				RetryWaitMax: time.Minute,
				RetryCount:   5,
			},
		},
		Observer: observerConfig{
			Enabled:                  true,
			DatabaseConnectionString: defaultConnectionString,
			TrustedPeers:             []string{},
			RetryInterval:            time.Minute,
			PollInterval:             time.Minute,
			W3S: w3sConfig{
				Token:        "",
				RetryWait:    10 * time.Second,
				RetryWaitMax: time.Minute,
				RetryCount:   5,
			},
		},
		Module: moduleConfig{
			Echo: echoConfig{
				Enabled: true,
			},
			QueryAsk: queryAskConfig{
				Enabled: true,
			},
			ThousandEyes: thousandEyesConfig{
				Enabled:  false,
				Token:    "",
				Username: "",
				Password: "",
			},
			Retrieval: retrievalConfig{
				Enabled:     true,
				TmpDir:      os.TempDir(),
				Timeout:     time.Minute,
				MinInterval: 10 * time.Minute,
				MaxJobs:     int64(1),
			},
		},
		Lotus: lotusConfig{
			URL:   "https://api.node.glif.io/",
			Token: "",
		},
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Warn().Str("config_path", configPath).Msg("config file does not exist, creating new one")
		log.Info().Msg("generating new peers for dispatcher and auditor as default")

		auditorKey, _, auditorPeer, err := role.GenerateNewPeer()
		if err != nil {
			return nil, errors.Wrap(err, "cannot generate auditor key")
		}

		dispatcherKey, _, dispatcherPeer, err := role.GenerateNewPeer()
		if err != nil {
			return nil, errors.Wrap(err, "cannot generate dispatcher key")
		}

		cfg.Auditor.PrivateKey = auditorKey
		cfg.Dispatcher.PrivateKey = dispatcherKey
		cfg.Auditor.TrustedPeers = []string{dispatcherPeer}
		cfg.Observer.TrustedPeers = []string{auditorPeer}

		log.Info().Str("config_path", configPath).Msg("writing defaults to config file")

		cfgStr, err := yaml.Marshal(cfg)
		if err != nil {
			return nil, errors.Wrap(err, "cannot marshal config to yaml")
		}

		err = os.WriteFile(configPath, cfgStr, 0o600)
		if err != nil {
			return nil, errors.Wrap(err, "cannot create an empty config file")
		}
	}

	viper.SetConfigFile(configPath)
	log.Debug().Str("config_path", configPath).Msg("reading config file")

	err := viper.ReadInConfig()
	if err != nil {
		return nil, errors.Wrap(err, "cannot read config file")
	}

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	err = viper.Unmarshal(&cfg)
	if err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal config")
	}

	return &cfg, nil
}

func deleteTaskHandler(c echo.Context, dispatcher taskRemover) error {
	id := c.Param("id")

	parsedID, err := uuid.Parse(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	err = dispatcher.Remove(c.Request().Context(), parsedID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusOK)
}

func postTaskHandler(c echo.Context, dispatcher taskCreator) error {
	var definition task.Definition

	err := c.Bind(&definition)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	err = dispatcher.Create(c.Request().Context(), &definition)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, definition)
}

func listTasksHandler(c echo.Context, dispatcher taskLister) error {
	definitions, err := dispatcher.List(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, definitions)
}

func subscribeToErrors(
	ctx context.Context,
	dispatcherErrorChannel <-chan error,
	auditorErrorChannel <-chan error,
	observerErrorChannel <-chan error,
) error {
	log := log3.With().Str("role", "main").Caller().Logger()
	log.Debug().Msg("subscribing to errors")
	select {
	case err := <-dispatcherErrorChannel:
		log.Error().Err(err).Msg("dispatcher error")
		return errors.Wrap(err, "dispatcher error")
	case err := <-auditorErrorChannel:
		log.Error().Err(err).Msg("auditor error")
		return errors.Wrap(err, "auditor error")
	case err := <-observerErrorChannel:
		log.Error().Err(err).Msg("observer error")
		return errors.Wrap(err, "observer error")
	case <-ctx.Done():
		log.Info().Msg("shutting down")
		return nil
	}
}

func setupAPI(dispatcher *dispatcher.Dispatcher, cfg *config) {
	api := echo.New()
	echoLogger := lecho.From(
		log3.Logger,
		lecho.WithLevel(log2.INFO),
		lecho.WithField("role", "http_api"),
		lecho.WithTimestamp(),
	)
	api.Logger = echoLogger
	api.Use(lecho.Middleware(lecho.Config{Logger: echoLogger}))
	api.Use(middleware.Recover())
	api.POST(
		createRoute, func(c echo.Context) error {
			return postTaskHandler(c, dispatcher)
		},
	)

	api.DELETE(
		deleteRoute, func(c echo.Context) error {
			return deleteTaskHandler(c, dispatcher)
		},
	)

	api.GET(
		listRoute, func(c echo.Context) error {
			return listTasksHandler(c, dispatcher)
		},
	)

	go func() {
		err := api.Start(cfg.Dispatcher.APIAddress)
		if err != nil {
			log := log3.With().Str("role", "main").Caller().Logger()
			log.Fatal().Err(err).Msg("cannot start dispatcher api")
			os.Exit(1)
		}
	}()
}

func run(configPath string) error {
	cfg, err := setConfig(configPath)
	if err != nil {
		return errors.Wrap(err, "cannot set config")
	}

	if cfg.Log.Pretty {
		log3.Logger = log3.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	log := log3.With().Str("role", "main").Caller().Logger()

	level, err := zerolog.ParseLevel(cfg.Log.Level)
	if err != nil {
		return errors.Wrap(err, "cannot parse log level")
	}

	zerolog.SetGlobalLevel(level)

	ctx := context.Background()
	var dispatcherErrorChannel, auditorErrorChannel, observerErrorChannel <-chan error
	var anyEnabled bool

	if cfg.Dispatcher.Enabled {
		log.Info().Msg("starting dispatcher")

		anyEnabled = true

		dispatcher, err := newDispatcher(ctx, cfg)
		if err != nil {
			return errors.Wrap(err, "cannot create dispatcher")
		}

		dispatcherErrorChannel = dispatcher.Start(ctx)

		log.Info().Msg("starting dispatcher api")
		setupAPI(dispatcher, cfg)
	}

	if cfg.Auditor.Enabled {
		log.Info().Msg("starting auditor")

		anyEnabled = true

		auditor, closer, err := newAuditor(ctx, cfg)
		if err != nil {
			return errors.Wrap(err, "cannot create auditor")
		}

		defer closer()

		auditorErrorChannel = auditor.Start(ctx)
	}

	if cfg.Observer.Enabled {
		log.Info().Msg("starting observer")

		anyEnabled = true

		observer, err := newObserver(cfg)
		if err != nil {
			return errors.Wrap(err, "cannot create observer")
		}

		observerErrorChannel = observer.Start(ctx)
	}

	if anyEnabled {
		return subscribeToErrors(ctx, dispatcherErrorChannel, auditorErrorChannel, observerErrorChannel)
	}

	return errors.New("no components enabled")
}

func main() {
	var configPath string
	log := log3.With().Str("role", "main").Caller().Logger()
	zerolog.DurationFieldUnit = time.Second
	app := &cli.App{
		Name: "validation-bot",
		Commands: []*cli.Command{
			{
				Name:  "run",
				Usage: "start running the validation bot",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "config",
						Aliases:     []string{"c"},
						Usage:       "path to the config file",
						Value:       "./config.yaml",
						Destination: &configPath,
					},
				},
				Action: func(c *cli.Context) error {
					return run(configPath)
				},
			},
			{
				Name:  "generate-peer",
				Usage: "generate a new peer id with private key",
				Action: func(c *cli.Context) error {
					privateStr, publicStr, peerStr, err := role.GenerateNewPeer()
					if err != nil {
						return errors.Wrap(err, "cannot generate new peer")
					}

					//nolint:forbidigo
					{
						fmt.Println("New peer generated using ed25519, keys are encoded in base64")
						fmt.Println("peer id:     ", peerStr)
						fmt.Println("public key:  ", publicStr)
						fmt.Println("private key: ", privateStr)
					}
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("")
		os.Exit(1)
	}
}

func newObserver(cfg *config) (*observer.Observer, error) {
	retryInterval := cfg.Observer.RetryInterval
	pollInterval := cfg.Observer.PollInterval
	retryWait := cfg.Observer.W3S.RetryWait
	retryWaitMax := cfg.Observer.W3S.RetryWaitMax
	retryCount := cfg.Observer.W3S.RetryCount
	config := store.W3StoreSubscriberConfig{
		RetryInterval: retryInterval,
		PollInterval:  pollInterval,
		RetryWait:     retryWait,
		RetryWaitMax:  retryWaitMax,
		RetryCount:    retryCount,
	}
	resultSubscriber := store.NewW3StoreSubscriber(config)
	connectionString := cfg.Observer.DatabaseConnectionString
	dblogger := role.GormLogger{
		Log: log3.With().CallerWithSkipFrameCount(1).Str("role", "sql").Logger(),
	}

	db, err := gorm.Open(postgres.Open(connectionString), &gorm.Config{Logger: dblogger})
	if err != nil {
		return nil, errors.Wrap(err, "cannot open database connection")
	}

	trustedPeers := cfg.Observer.TrustedPeers
	peers := make([]peer.ID, len(trustedPeers))

	for i, trustedPeer := range trustedPeers {
		peerID, err := peer.Decode(trustedPeer)
		if err != nil {
			return nil, errors.Wrap(err, "cannot decode peer id")
		}

		peers[i] = peerID
	}

	observer, err := observer.NewObserver(db, resultSubscriber, peers)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create observer")
	}

	return observer, nil
}

type Closer func()

//nolint:funlen,cyclop
func newAuditor(ctx context.Context, cfg *config) (*auditor.Auditor, Closer, error) {
	libp2p, err := role.NewLibp2pHost(cfg.Auditor.PrivateKey, cfg.Auditor.ListenAddr)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot create pubsub config")
	}

	token := cfg.Auditor.W3S.Token
	if token == "" {
		return nil, nil, errors.New("auditor.w3s_token is empty")
	}

	taskSubscriber, err := task.NewLibp2pTaskSubscriber(ctx, *libp2p, cfg.Auditor.TopicNames)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot create task subscriber")
	}

	config := store.W3StorePublisherConfig{
		Token:        token,
		PrivateKey:   cfg.Auditor.PrivateKey,
		RetryWait:    cfg.Auditor.W3S.RetryWait,
		RetryWaitMax: cfg.Auditor.W3S.RetryWaitMax,
		RetryCount:   cfg.Auditor.W3S.RetryCount,
	}

	resultPublisher, err := store.NewW3StorePublisher(ctx, config)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot create result publisher")
	}

	trustedPeers := cfg.Auditor.TrustedPeers
	peers := make([]peer.ID, len(trustedPeers))

	for i, trustedPeer := range trustedPeers {
		peerID, err := peer.Decode(trustedPeer)
		if err != nil {
			return nil, nil, errors.Wrap(err, "cannot decode peer id")
		}

		peers[i] = peerID
	}

	modules := map[string]module.AuditorModule{}
	var lotusAPI api.Gateway
	var closer Closer = func() {}

	if cfg.Module.Echo.Enabled {
		echoModule := echo_module.NewEchoAuditor()
		modules[task.Echo] = &echoModule
	}

	if cfg.Module.Retrieval.Enabled || cfg.Module.ThousandEyes.Enabled {
		var header http.Header
		if cfg.Lotus.Token != "" {
			header = http.Header{
				"Authorization": []string{"Bearer " + cfg.Lotus.Token},
			}
		}

		var clientCloser jsonrpc.ClientCloser

		lotusAPI, clientCloser, err = client.NewGatewayRPCV1(ctx, cfg.Lotus.URL, header)
		if err != nil {
			return nil, nil, errors.Wrap(err, "cannot create lotus api")
		}

		closer = func() {
			clientCloser()
		}
	}

	if cfg.Module.QueryAsk.Enabled {
		queryAskModule := queryask.NewAuditor(libp2p, lotusAPI)
		modules[task.QueryAsk] = &queryAskModule
	}

	if cfg.Module.ThousandEyes.Enabled {
		agents := []string{}
		agentIDs := make([]thousandeyes.AgentID, len(agents))

		for i, agent := range agents {
			agentID, err := strconv.Atoi(agent)
			if err != nil {
				return nil, nil, errors.Wrap(err, "cannot parse thousandeyes agent id")
			}
			//nolint:gosec
			agentIDs[i] = thousandeyes.AgentID{AgentID: int32(agentID)}
		}

		switch {
		case cfg.Module.ThousandEyes.Token != "":
			temodule := thousandeyes.NewAuditorModuleWithAuthToken(
				lotusAPI,
				cfg.Module.ThousandEyes.Token,
				agentIDs,
			)
			modules[task.ThousandEyes] = &temodule
		case cfg.Module.ThousandEyes.Username != "" && cfg.Module.ThousandEyes.Password != "":
			temodule := thousandeyes.NewAuditorModuleWithBasicAuth(
				lotusAPI,
				cfg.Module.ThousandEyes.Username,
				cfg.Module.ThousandEyes.Password,
				agentIDs,
			)
			modules[task.ThousandEyes] = &temodule
		default:
			return nil, nil, errors.New("thousandeyes module enabled but no authentication provided")
		}
	}

	if cfg.Module.Retrieval.Enabled {
		tmpDir := cfg.Module.Retrieval.TmpDir
		timeout := cfg.Module.Retrieval.Timeout
		maxJobs := cfg.Module.Retrieval.MaxJobs
		graphsync := retrieval.GraphSyncRetrieverBuilderImpl{
			LotusAPI: lotusAPI,
			BaseDir:  tmpDir,
		}
		retrievalModule := retrieval.NewAuditor(&graphsync, timeout, maxJobs)
		modules[task.Retrieval] = &retrievalModule
	}

	auditor, err := auditor.NewAuditor(
		auditor.Config{
			ResultPublisher: resultPublisher,
			TaskSubscriber:  taskSubscriber,
			TrustedPeers:    peers,
			Modules:         modules,
		},
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot create auditor")
	}

	return auditor, closer, nil
}

func newDispatcher(ctx context.Context, cfg *config) (*dispatcher.Dispatcher, error) {
	connectionString := cfg.Dispatcher.DatabaseConnectionString
	dblogger := role.GormLogger{
		Log: log3.With().Str("role", "sql").Logger(),
	}

	db, err := gorm.Open(postgres.Open(connectionString), &gorm.Config{Logger: dblogger})
	if err != nil {
		return nil, errors.Wrap(err, "cannot open database connection")
	}

	err = db.WithContext(ctx).AutoMigrate(&task.Definition{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot migrate task definitions")
	}

	libp2p, err := role.NewLibp2pHost(
		cfg.Dispatcher.PrivateKey,
		cfg.Dispatcher.ListenAddr,
	)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create pubsub config")
	}

	taskPublisher, err := task.NewLibp2pTaskPublisher(ctx, *libp2p, cfg.Dispatcher.TopicName)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create task publisher")
	}

	modules := map[string]module.DispatcherModule{}

	if cfg.Module.Echo.Enabled {
		echoModule := echo_module.Dispatcher{
			SimpleDispatcher: module.SimpleDispatcher{},
		}
		modules[task.Echo] = &echoModule
	}

	if cfg.Module.QueryAsk.Enabled {
		queryAskModule := queryask.Dispatcher{
			SimpleDispatcher: module.SimpleDispatcher{},
		}
		modules[task.QueryAsk] = &queryAskModule
	}

	if cfg.Module.ThousandEyes.Enabled {
		teModule := thousandeyes.Dispatcher{
			SimpleDispatcher: module.SimpleDispatcher{},
		}
		modules[task.ThousandEyes] = &teModule
	}

	if cfg.Module.Retrieval.Enabled {
		retrievalModule := retrieval.NewDispatcher(cfg.Module.Retrieval.MinInterval)
		modules[task.Retrieval] = &retrievalModule
	}

	dispatcherConfig := dispatcher.Config{
		DB:            db,
		TaskPublisher: taskPublisher,
		CheckInterval: cfg.Dispatcher.CheckInterval,
		Modules:       modules,
	}

	dispatcher, err := dispatcher.NewDispatcher(dispatcherConfig)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create dispatcher")
	}
	return dispatcher, nil
}

type MockTaskRemover struct {
	mock.Mock
}

func (m *MockTaskRemover) Remove(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type MockTaskCreator struct {
	mock.Mock
}

func (m *MockTaskCreator) Create(ctx context.Context, taskDef *task.Definition) error {
	args := m.Called(ctx, taskDef)
	return args.Error(0)
}

type MockTaskLister struct {
	mock.Mock
}

//nolint:all
func (m *MockTaskCreator) List(ctx context.Context) ([]task.Definition, error) {
	args := m.Called(ctx)
	return args.Get(0).([]task.Definition), args.Error(1)
}
