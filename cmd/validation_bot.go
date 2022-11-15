package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"validation-bot/module/indexprovider"
	"validation-bot/module/traceroute"
	"validation-bot/role/trust"

	"validation-bot/module/queryask"
	"validation-bot/module/retrieval"
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
	"golang.org/x/exp/slices"
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
func setConfig(ctx context.Context, configPath string) (*config, error) {
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
			AuthenticationTokens:     []string{},
			Jitter:                   time.Minute,
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
			Retrieval: retrievalConfig{
				Enabled: true,
				TmpDir:  os.TempDir(),
				Timeout: 30 * time.Second,
				MaxJobs: int64(1),
				LocationFilter: module.LocationFilterConfig{
					Continent: nil,
					Country:   nil,
				},
			},
			Traceroute: tracerouteConfig{
				Enabled: true,
				UseSudo: false,
			},
			IndexProvider: indexProviderConfig{
				Enabled: true,
			},
		},
		Lotus: lotusConfig{
			URL:                             "https://api.node.glif.io/",
			Token:                           "",
			StateMarketDealsURL:             "https://marketdeals.s3.amazonaws.com/StateMarketDeals.json",
			StateMarketDealsRefreshInterval: 4 * time.Hour,
			SQLInsertBatchSize:              1000,
		},
	}

	//nolint:nestif
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
		cfg.Auditor.TrustedPeers = []string{dispatcherPeer.String()}
		cfg.Observer.TrustedPeers = []string{dispatcherPeer.String()}

		if os.Getenv("AUDITOR_W3S_TOKEN") == "" {
			log.Warn().Msg("AUDITOR_W3S_TOKEN env variable is not set, skip publishing auditor peer to w3s")
		} else {
			publisher, err := store.NewW3StorePublisher(
				ctx, store.W3StorePublisherConfig{
					Token:        os.Getenv("AUDITOR_W3S_TOKEN"),
					PrivateKey:   dispatcherKey,
					RetryWait:    time.Second,
					RetryWaitMax: time.Minute,
					RetryCount:   10,
				},
			)
			if err != nil {
				return nil, errors.Wrap(err, "cannot create publisher")
			}

			err = trust.AddNewPeer(ctx, publisher, auditorPeer)
			if err != nil {
				return nil, errors.Wrap(err, "cannot add auditor peer as a trusted peer")
			}
		}

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

	handleAuth := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if len(cfg.Dispatcher.AuthenticationTokens) > 0 {
				auth := c.Request().Header.Get("Authorization")
				if auth == "" ||
					strings.ToLower(auth[:7]) != "bearer " ||
					!slices.Contains(cfg.Dispatcher.AuthenticationTokens, auth[7:]) {
					return echo.NewHTTPError(http.StatusUnauthorized, "invalid auth token")
				}
			}
			return next(c)
		}
	}

	api.Use(handleAuth)

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

func run(ctx context.Context, configPath string) error {
	cfg, err := setConfig(ctx, configPath)
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

	var anyEnabled bool

	if cfg.Dispatcher.Enabled {
		log.Info().Msg("starting dispatcher")

		anyEnabled = true

		dispatcher, err := newDispatcher(ctx, cfg)
		if err != nil {
			return errors.Wrap(err, "cannot create dispatcher")
		}

		dispatcher.Start(ctx)

		log.Info().Msg("starting dispatcher api")
		//nolint:contextcheck
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

		auditor.Start(ctx)
	}

	if cfg.Observer.Enabled {
		log.Info().Msg("starting observer")

		anyEnabled = true

		observer, err := newObserver(cfg)
		if err != nil {
			return errors.Wrap(err, "cannot create observer")
		}

		observer.Start(ctx)
	}

	if !anyEnabled {
		return errors.New("no components enabled")
	}

	<-ctx.Done()
	log.Info().Msg("shutting down")
	return nil
}

//nolint:funlen,forbidigo,dupl
func main() {
	var configPath string
	var privateKey string
	var w3sToken string
	var peerIDStr string
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
					return run(c.Context, configPath)
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
						fmt.Println("peer id:     ", peerStr.String())
						fmt.Println("public key:  ", publicStr)
						fmt.Println("private key: ", privateStr)
					}
					return nil
				},
			},
			{
				Name:  "add-trusted-peer",
				Usage: "add a new trusted peer and publish to the network via W3Sname",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "private-key",
						Aliases:     []string{"k"},
						Usage:       "private key of the trustor",
						Destination: &privateKey,
						Required:    true,
					},
					&cli.StringFlag{
						Name:        "w3s-token",
						Aliases:     []string{"t"},
						Usage:       "token for web3.storage",
						Destination: &w3sToken,
						Required:    true,
					},
					&cli.StringFlag{
						Name:        "peerid",
						Aliases:     []string{"p"},
						Usage:       "peerIDStr to trust",
						Destination: &peerIDStr,
						Required:    true,
					},
				},
				Action: func(c *cli.Context) error {
					//nolint:gomnd
					publisher, err := store.NewW3StorePublisher(
						c.Context, store.W3StorePublisherConfig{
							Token:        w3sToken,
							PrivateKey:   privateKey,
							RetryWait:    time.Second,
							RetryWaitMax: time.Minute,
							RetryCount:   10,
						},
					)
					if err != nil {
						return errors.Wrap(err, "cannot create publisher")
					}

					peerID, err := peer.Decode(peerIDStr)
					if err != nil {
						return errors.Wrap(err, "cannot decode peer id")
					}

					err = trust.AddNewPeer(c.Context, publisher, peerID)
					if err != nil {
						return errors.Wrap(err, "cannot publish record to trust new peer")
					}

					return nil
				},
			},
			{
				Name:  "revoke-trusted-peer",
				Usage: "add a trusted peer and publish to the network via W3Sname",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "private-key",
						Aliases:     []string{"k"},
						Usage:       "private key of the trustor",
						Destination: &privateKey,
						Required:    true,
					},
					&cli.StringFlag{
						Name:        "w3s-token",
						Aliases:     []string{"t"},
						Usage:       "token for web3.storage",
						Destination: &w3sToken,
						Required:    true,
					},
					&cli.StringFlag{
						Name:        "peerid",
						Aliases:     []string{"p"},
						Usage:       "peerIDStr to trust",
						Destination: &peerIDStr,
						Required:    true,
					},
				},
				Action: func(c *cli.Context) error {
					//nolint:gomnd
					publisher, err := store.NewW3StorePublisher(
						c.Context, store.W3StorePublisherConfig{
							Token:        w3sToken,
							PrivateKey:   privateKey,
							RetryWait:    time.Second,
							RetryWaitMax: time.Minute,
							RetryCount:   10,
						},
					)
					if err != nil {
						return errors.Wrap(err, "cannot create publisher")
					}

					peerID, err := peer.Decode(peerIDStr)
					if err != nil {
						return errors.Wrap(err, "cannot decode peer id")
					}

					err = trust.RevokePeer(c.Context, publisher, peerID)
					if err != nil {
						return errors.Wrap(err, "cannot publish record to revoke peer")
					}

					return nil
				},
			},
			{
				Name:  "list-trusted-peers",
				Usage: "list all published trusted peers",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "peerid",
						Aliases:     []string{"p"},
						Usage:       "peerIDStr of the trustor",
						Destination: &peerIDStr,
						Required:    true,
					},
				},
				Action: func(c *cli.Context) error {
					//nolint:gomnd
					subscriber := store.NewW3StoreSubscriber(
						store.W3StoreSubscriberConfig{
							RetryInterval: time.Second,
							PollInterval:  time.Minute,
							RetryWait:     time.Second,
							RetryWaitMax:  time.Minute,
							RetryCount:    10,
						},
					)

					peerID, err := peer.Decode(peerIDStr)
					if err != nil {
						return errors.Wrap(err, "cannot decode peer id")
					}

					peers, err := trust.ListPeers(c.Context, subscriber, peerID)
					if err != nil {
						return errors.Wrap(err, "cannot list trusted peers")
					}

					fmt.Println("Trusted peers:")
					for peerStr, valid := range peers {
						fmt.Printf("%s - Valid: %v\n", peerStr, valid)
					}

					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("")
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

	if cfg.Module.Echo.Enabled {
		echoModule := echo_module.NewEchoAuditor()
		modules[task.Echo] = &echoModule
	}

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

	if cfg.Module.QueryAsk.Enabled {
		queryAskModule := queryask.NewAuditor(libp2p, lotusAPI)
		modules[task.QueryAsk] = &queryAskModule
	}

	if cfg.Module.Retrieval.Enabled {
		tmpDir := cfg.Module.Retrieval.TmpDir
		timeout := cfg.Module.Retrieval.Timeout
		maxJobs := cfg.Module.Retrieval.MaxJobs
		graphsync := retrieval.GraphSyncRetrieverBuilderImpl{
			LotusAPI: lotusAPI,
			BaseDir:  tmpDir,
		}

		retrievalModule, err := retrieval.NewAuditor(
			lotusAPI,
			&graphsync,
			timeout,
			maxJobs,
			cfg.Module.Retrieval.LocationFilter,
		)
		if err != nil {
			return nil, nil, errors.Wrap(err, "cannot create retrieval module")
		}

		modules[task.Retrieval] = retrievalModule
	}

	if cfg.Module.Traceroute.Enabled {
		tracerouteModule := traceroute.NewAuditor(lotusAPI, cfg.Module.Traceroute.UseSudo)
		modules[task.Traceroute] = &tracerouteModule
	}

	if cfg.Module.IndexProvider.Enabled {
		indexProviderModule := indexprovider.NewAuditor(lotusAPI)
		modules[task.IndexProvider] = &indexProviderModule
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

	return auditor, Closer(clientCloser), nil
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
			NoopValidator:    module.NoopValidator{},
		}
		modules[task.Echo] = &echoModule
	}

	if cfg.Module.QueryAsk.Enabled {
		queryAskModule := queryask.Dispatcher{
			SimpleDispatcher: module.SimpleDispatcher{},
			NoopValidator:    module.NoopValidator{},
		}
		modules[task.QueryAsk] = &queryAskModule
	}

	if cfg.Module.Traceroute.Enabled {
		tracerouteModule := traceroute.Dispatcher{
			SimpleDispatcher: module.SimpleDispatcher{},
			NoopValidator:    module.NoopValidator{},
		}
		modules[task.Traceroute] = &tracerouteModule
	}

	if cfg.Module.IndexProvider.Enabled {
		indexProviderModule := indexprovider.Dispatcher{
			SimpleDispatcher: module.SimpleDispatcher{},
			NoopValidator:    module.NoopValidator{},
		}
		modules[task.IndexProvider] = &indexProviderModule
	}

	if cfg.Module.Retrieval.Enabled {
		var header http.Header
		if cfg.Lotus.Token != "" {
			header = http.Header{
				"Authorization": []string{"Bearer " + cfg.Lotus.Token},
			}
		}

		var clientCloser jsonrpc.ClientCloser

		lotusAPI, clientCloser, err := client.NewGatewayRPCV1(ctx, cfg.Lotus.URL, header)
		if err != nil {
			return nil, errors.Wrap(err, "cannot create lotus api")
		}

		defer clientCloser()

		dealResolver, err := module.NewGlifDealStatesResolver(
			ctx,
			db,
			lotusAPI, cfg.Lotus.StateMarketDealsURL,
			cfg.Lotus.StateMarketDealsRefreshInterval,
			cfg.Lotus.SQLInsertBatchSize,
		)
		if err != nil {
			return nil, errors.Wrap(err, "cannot create deal resolver")
		}

		retrievalModule := retrieval.NewDispatcher(dealResolver)
		modules[task.Retrieval] = &retrievalModule
	}

	dispatcherConfig := dispatcher.Config{
		DB:            db,
		TaskPublisher: taskPublisher,
		CheckInterval: cfg.Dispatcher.CheckInterval,
		Modules:       modules,
		Jitter:        cfg.Dispatcher.Jitter,
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
