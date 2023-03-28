package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"validation-bot/module"
	echo_module "validation-bot/module/echo"
	"validation-bot/module/indexprovider"
	"validation-bot/module/queryask"
	"validation-bot/module/retrieval"
	"validation-bot/module/traceroute"
	"validation-bot/role"
	"validation-bot/role/auditor"
	"validation-bot/role/dispatcher"
	"validation-bot/role/observer"
	"validation-bot/role/trust"
	"validation-bot/rpcserver"
	"validation-bot/store"
	"validation-bot/task"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	log2 "github.com/labstack/gommon/log"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	log3 "github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"github.com/ziflex/lecho/v3"
	"go.uber.org/dig"
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
	Remove(ctx context.Context, id task.DefinitionID) error
}

//nolint:gomnd,funlen,cyclop
func setConfig(ctx context.Context, configPath string) (*config, error) {
	log := log3.With().Str("role", "main").Caller().Logger()
	defaultConnectionString := "host=localhost port=5432 user=postgres password=postgres dbname=postgres"
	retreivalTimeout := 30 * time.Second

	cfg := config{
		Log: logConfig{
			Pretty: true,
			Level:  "debug",
		},
		Topic: topicConfig{
			TopicName: "/filecoin/validation_bot/dev",
		},
		Database: databaseConfig{
			ConnectionString: defaultConnectionString,
		},
		Trust: trustConfig{
			Trustors:      []string{},
			RetryInterval: 10 * time.Second,
			PollInterval:  time.Minute,
		},
		W3S: w3sConfig{
			Token:                   "",
			ClientRetryWait:         10 * time.Second,
			ClientRetryWaitMax:      time.Minute,
			ClientRetryCount:        5,
			SubscriberRetryInterval: 10 * time.Second,
			SubscriberPollInterval:  time.Minute,
		},
		Dispatcher: dispatcherConfig{
			Enabled:              true,
			PrivateKey:           "",
			APIAddress:           ":8000",
			ListenAddr:           "/ip4/0.0.0.0/tcp/7998",
			CheckInterval:        time.Minute,
			AuthenticationTokens: []string{},
			Jitter:               time.Minute,
		},
		Auditor: auditorConfig{
			Enabled:     true,
			PrivateKey:  "",
			ListenAddr:  "/ip4/0.0.0.0/tcp/7999",
			BiddingWait: 10 * time.Second,
			ClientRPC: rpcClientConfig{
				Timeout: 15 * time.Minute,
				BaseDir: os.TempDir(),
				ExecPath: func() string {
					wd, err := os.Getwd()
					if err != nil {
						panic(err)
					}

					if _, err := os.Stat(filepath.Join(wd, "app", "validation_bot")); !os.IsNotExist(err) {
						wd = filepath.Join(wd, "app")
					}

					// nolint:forbidigo
					fmt.Printf("wd: %s", wd)
					return wd
				}(),
			},
		},
		Observer: observerConfig{
			Enabled:       true,
			RetryInterval: time.Minute,
			PollInterval:  time.Minute,
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
				Timeout: retreivalTimeout,
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
		IPInfo: ipInfoConfig{
			Token: "",
		},
		Lotus: lotusConfig{
			URL:   "https://api.node.glif.io/",
			Token: "",
		},
		DealStates: dealStatesConfig{
			DownloadURL:        "https://marketdeals.s3.amazonaws.com/StateMarketDeals.json",
			RefreshInterval:    4 * time.Hour,
			SQLInsertBatchSize: 1000,
		},
	}

	//nolint:nestif
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Warn().Str("config_path", configPath).Msg("config file does not exist, creating new one")

		if os.Getenv("DISPATCHER_PRIVATEKEY") == "" && os.Getenv("AUDITOR_PRIVATEKEY") == "" {
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
			cfg.Trust.Trustors = []string{dispatcherPeer.String()}

			if os.Getenv("W3S_TOKEN") == "" {
				log.Warn().Msg("W3S_TOKEN env variable is not set, skip publishing auditor peer to w3s")
				log.Warn().Msgf(
					"To fix this, run validation-bot add-trusted-peer -k %s -p %s -k <W3S_TOKEN>",
					dispatcherKey,
					auditorPeer.String(),
				)
			} else {
				publisher, err := store.NewW3StorePublisher(
					ctx, store.W3StorePublisherConfig{
						Token:        os.Getenv("W3S_TOKEN"),
						PrivateKey:   dispatcherKey,
						RetryWait:    time.Second,
						RetryWaitMax: time.Minute,
						RetryCount:   10,
					},
				)
				if err != nil {
					return nil, errors.Wrap(err, "cannot create publisher")
				}

				subscriber := store.NewW3StoreSubscriber(
					store.W3StoreSubscriberConfig{
						RetryInterval: time.Second,
						PollInterval:  time.Second,
						RetryWait:     time.Second,
						RetryWaitMax:  time.Second,
						RetryCount:    3,
					},
				)

				err = trust.ModifyPeers(
					ctx,
					publisher,
					subscriber,
					trust.Create,
					dispatcherPeer,
					[]peer.ID{auditorPeer},
					time.Second,
				)
				if err != nil {
					return nil, errors.Wrap(err, "cannot add auditor peer as a trusted peer")
				}
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

//nolint:wrapcheck,funlen,gocyclo,cyclop,maintidx
func setupDependencies(ctx context.Context, container *dig.Container, configPath string) (*config, error) {
	log := log3.With().Str("role", "main").Caller().Logger()

	cfg, err := setConfig(ctx, configPath)
	if err != nil {
		return nil, errors.Wrap(err, "cannot set config")
	}

	if cfg.Log.Pretty {
		log3.Logger = log3.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	level, err := zerolog.ParseLevel(cfg.Log.Level)
	if err != nil {
		return nil, errors.Wrap(err, "cannot parse log level")
	}

	zerolog.SetGlobalLevel(level)

	// DI: api.Gateway
	err = container.Provide(
		func() (api.Gateway, error) {
			var header http.Header
			if cfg.Lotus.Token != "" {
				header = http.Header{
					"Authorization": []string{"Bearer " + cfg.Lotus.Token},
				}
			}

			lotusAPI, _, err := client.NewGatewayRPCV1(ctx, cfg.Lotus.URL, header)
			return lotusAPI, err
		},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot provide lotus api")
	}

	// DI: gorm.DB
	err = container.Provide(
		func() (*gorm.DB, error) {
			connectionString := cfg.Database.ConnectionString
			dblogger := role.GormLogger{
				Log: log3.With().Str("role", "sql").Logger(),
			}

			db, err := gorm.Open(postgres.Open(connectionString), &gorm.Config{Logger: dblogger})
			if err != nil {
				return nil, errors.Wrap(err, "cannot open database connection")
			}

			return db, nil
		},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot provide dispatcher database")
	}

	// DI: module.DealStatesResolver
	err = container.Provide(
		func(db *gorm.DB, lotusAPI api.Gateway) (module.DealStatesResolver, error) {
			resolver, err := module.NewGlifDealStatesResolver(
				db,
				lotusAPI,
				cfg.DealStates.DownloadURL,
				cfg.DealStates.RefreshInterval,
				cfg.DealStates.SQLInsertBatchSize,
			)
			if err != nil {
				resolver.Start(context.Background())
			}

			return resolver, err
		},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot provide deal states resolver")
	}

	// DI: host.Host - for dispatcher
	err = container.Provide(
		func() (host.Host, error) {
			return role.NewLibp2pHost(
				cfg.Dispatcher.PrivateKey,
				cfg.Dispatcher.ListenAddr,
			)
		},
		dig.Name("dispatcher_libp2p"),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot provide dispatcher Libp2p host")
	}

	// DI: host.Host - for auditor
	err = container.Provide(
		func() (host.Host, error) {
			return role.NewLibp2pHost(
				cfg.Auditor.PrivateKey,
				cfg.Auditor.ListenAddr,
			)
		},
		dig.Name("auditor_libp2p"),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot provide auditor Libp2p host")
	}

	// DI: task.PublisherSubscriber - for dispatcher
	type DispatcherTaskPublisherSubscriberParams struct {
		dig.In
		Libp2p host.Host `name:"dispatcher_libp2p"`
	}

	err = container.Provide(
		func(params DispatcherTaskPublisherSubscriberParams) (task.PublisherSubscriber, error) {
			return task.NewLibp2pPublisherSubscriber(ctx, params.Libp2p, cfg.Topic.TopicName)
		},
		dig.Name("dispatcher_task_publisher_subscriber"),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot provide dispatcher task publisher subscriber")
	}

	// DI: task.PublisherSubscriber - for auditor
	type AuditorTaskPublisherSubscriberParams struct {
		dig.In
		Libp2p host.Host `name:"auditor_libp2p"`
	}

	err = container.Provide(
		func(params AuditorTaskPublisherSubscriberParams) (task.PublisherSubscriber, error) {
			return task.NewLibp2pPublisherSubscriber(ctx, params.Libp2p, cfg.Topic.TopicName)
		},
		dig.Name("auditor_task_publisher_subscriber"),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot provide auditor task publisher subscriber")
	}

	// DI: store.Publisher - for auditor
	err = container.Provide(
		func() (store.Publisher, error) {
			token := cfg.W3S.Token
			if token == "" {
				return nil, errors.New("W3S token is empty")
			}

			config := store.W3StorePublisherConfig{
				Token:        token,
				PrivateKey:   cfg.Auditor.PrivateKey,
				RetryWait:    cfg.W3S.ClientRetryWait,
				RetryWaitMax: cfg.W3S.ClientRetryWaitMax,
				RetryCount:   cfg.W3S.ClientRetryCount,
			}

			return store.NewW3StorePublisher(ctx, config)
		},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot provide store publisher")
	}

	// DI: store.Subscriber - for observer
	err = container.Provide(
		func() store.Subscriber {
			retryInterval := cfg.W3S.SubscriberRetryInterval
			pollInterval := cfg.W3S.SubscriberPollInterval
			retryWait := cfg.W3S.ClientRetryWait
			retryWaitMax := cfg.W3S.ClientRetryWaitMax
			retryCount := cfg.W3S.ClientRetryCount
			config := store.W3StoreSubscriberConfig{
				RetryInterval: retryInterval,
				PollInterval:  pollInterval,
				RetryWait:     retryWait,
				RetryWaitMax:  retryWaitMax,
				RetryCount:    retryCount,
			}
			return store.NewW3StoreSubscriber(config)
		},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot provide store subscriber")
	}

	// Modules
	type DispatcherModuleResult struct {
		dig.Out
		Module module.DispatcherModule `group:"dispatcher_module"`
	}

	type AuditorModuleResult struct {
		dig.Out
		Module module.AuditorModule `group:"auditor_module"`
	}

	// DI: echo module
	if cfg.Module.Echo.Enabled {
		err = container.Provide(
			func() DispatcherModuleResult {
				return DispatcherModuleResult{
					Module: echo_module.Dispatcher{
						SimpleDispatcher: module.SimpleDispatcher{},
						NoopValidator:    module.NoopValidator{},
					},
				}
			},
		)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot provide echo dispatcher module")
		}

		err = container.Provide(
			func() AuditorModuleResult {
				return AuditorModuleResult{
					Module: echo_module.NewEchoAuditor(),
				}
			},
		)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot provide echo auditor module")
		}
	}

	// DI: query ask module
	if cfg.Module.QueryAsk.Enabled {
		err = container.Provide(
			func() DispatcherModuleResult {
				return DispatcherModuleResult{
					Module: queryask.Dispatcher{
						SimpleDispatcher: module.SimpleDispatcher{},
						NoopValidator:    module.NoopValidator{},
					},
				}
			},
		)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot provide query ask dispatcher module")
		}

		err = container.Provide(
			func(lotusAPI api.Gateway) AuditorModuleResult {
				return AuditorModuleResult{
					Module: queryask.NewAuditor(lotusAPI),
				}
			},
		)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot provide query ask auditor module")
		}
	}

	// DI: index provider module
	if cfg.Module.IndexProvider.Enabled {
		err = container.Provide(
			func() DispatcherModuleResult {
				return DispatcherModuleResult{
					Module: indexprovider.Dispatcher{
						SimpleDispatcher: module.SimpleDispatcher{},
						NoopValidator:    module.NoopValidator{},
					},
				}
			},
		)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot provide index provider dispatcher module")
		}

		err = container.Provide(
			func(lotusAPI api.Gateway) AuditorModuleResult {
				return AuditorModuleResult{
					Module: indexprovider.NewAuditor(lotusAPI),
				}
			},
		)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot provide index provider auditor module")
		}
	}

	// DI: traceroute module
	if cfg.Module.Traceroute.Enabled {
		err = container.Provide(
			func() DispatcherModuleResult {
				return DispatcherModuleResult{
					Module: traceroute.Dispatcher{
						SimpleDispatcher: module.SimpleDispatcher{},
						NoopValidator:    module.NoopValidator{},
					},
				}
			},
		)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot provide traceroute dispatcher module")
		}

		err = container.Provide(
			func(lotusAPI api.Gateway) AuditorModuleResult {
				return AuditorModuleResult{
					Module: traceroute.NewAuditor(lotusAPI, cfg.Module.Traceroute.UseSudo),
				}
			},
		)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot provide trace route auditor module")
		}
	}

	// DI: ipinfo resolver module
	err = container.Provide(
		func() (module.IPInfoResolver, error) {
			return module.NewIPInfoResolver(cfg.IPInfo.Token)
		},
	)

	if err != nil {
		log.Fatal().Err(err).Msg("cannot provide ipinfo resolver module")
	}

	// DI: retrieval module
	if cfg.Module.Retrieval.Enabled {
		err = container.Provide(
			func(dealResolver module.DealStatesResolver) DispatcherModuleResult {
				return DispatcherModuleResult{
					Module: retrieval.NewDispatcher(dealResolver),
				}
			},
		)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot provide retrieval dispatcher module")
		}

		err = container.Provide(
			func(lotusAPI api.Gateway, ipinfoResolver module.IPInfoResolver) (AuditorModuleResult, error) {
				auditor, err := retrieval.NewAuditor(
					lotusAPI,
					retrieval.GraphSyncRetrieverBuilderImpl{
						LotusAPI: lotusAPI,
						BaseDir:  cfg.Module.Retrieval.TmpDir,
					},
					retrieval.BitswapRetrieverBuilder{},
					cfg.Module.Retrieval.Timeout,
					cfg.Module.Retrieval.MaxJobs,
					cfg.Module.Retrieval.LocationFilter,
					ipinfoResolver,
				)
				if err != nil {
					return AuditorModuleResult{}, err
				}

				return AuditorModuleResult{
					Module: auditor,
				}, nil
			},
		)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot provide retrieval auditor module")
		}
	}

	// DI: dispatcher.Dispatcher
	type DispatcherParams struct {
		dig.In
		DB                      *gorm.DB
		TaskPublisherSubscriber task.PublisherSubscriber  `name:"dispatcher_task_publisher_subscriber"`
		Modules                 []module.DispatcherModule `group:"dispatcher_module"`
	}

	err = container.Provide(
		func(params DispatcherParams) (*dispatcher.Dispatcher, error) {
			modules := make(map[string]module.DispatcherModule)
			for _, m := range params.Modules {
				modules[m.Type()] = m
			}

			dispatcherConfig := dispatcher.Config{
				DB:                      params.DB,
				TaskPublisherSubscriber: params.TaskPublisherSubscriber,
				CheckInterval:           cfg.Dispatcher.CheckInterval,
				Modules:                 modules,
				Jitter:                  cfg.Dispatcher.Jitter,
			}

			return dispatcher.NewDispatcher(dispatcherConfig)
		},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot provide dispatcher")
	}

	// DI: trusted dispatcher peers
	err = container.Provide(
		func() ([]peer.ID, error) {
			trustedPeers := cfg.Trust.Trustors
			peers := make([]peer.ID, len(trustedPeers))

			for i, trustedPeer := range trustedPeers {
				peerID, err := peer.Decode(trustedPeer)
				if err != nil {
					return nil, errors.Wrap(err, "cannot decode peer id")
				}

				peers[i] = peerID
			}
			return peers, nil
		},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot provide trusted peers")
	}

	// DI: trust.Manager
	err = container.Provide(
		func(trustedPeers []peer.ID, resultSubscriber store.Subscriber) *trust.Manager {
			manager := trust.NewManager(trustedPeers, resultSubscriber, cfg.Trust.RetryInterval, cfg.Trust.PollInterval)
			manager.Start(context.Background())
			return manager
		},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot provide trust manager")
	}

	err = container.Provide(
		func() *auditor.ClientRPC {
			return auditor.NewClientRPC(auditor.ClientConfig{
				BaseDir:  cfg.Auditor.ClientRPC.BaseDir,
				Timeout:  cfg.Auditor.ClientRPC.Timeout,
				ExecPath: cfg.Auditor.ClientRPC.ExecPath,
			})
		},
		dig.Name("auditor_rpc_client"),
	)

	if err != nil {
		log.Fatal().Err(err).Msg("cannot provide rpc client")
	}

	type RPCServerParams struct {
		dig.In
		Modules []module.AuditorModule `group:"auditor_module"`
	}

	err = container.Provide(
		func(params RPCServerParams) (*rpcserver.RPCServer, error) {
			modules := make(map[string]module.AuditorModule)
			for _, m := range params.Modules {
				modules[m.Type()] = m
			}

			return rpcserver.NewRPCServer(
				rpcserver.Config{
					Modules: modules,
				},
			), nil
		},
	)

	if err != nil {
		log.Fatal().Err(err).Msg("cannot provide rpc validator")
	}

	// DI: auditor.Auditor
	type AuditorParams struct {
		dig.In
		Modules                 []module.AuditorModule `group:"auditor_module"`
		Libp2p                  host.Host              `name:"auditor_libp2p"`
		TrustManager            *trust.Manager
		ResultPublisher         store.Publisher
		TaskPublisherSubscriber task.PublisherSubscriber `name:"auditor_task_publisher_subscriber"`
		ClientRPC               *auditor.ClientRPC       `name:"auditor_rpc_client"`
	}

	err = container.Provide(
		func(params AuditorParams, resultSubscriber store.Subscriber) (*auditor.Auditor, error) {
			trustedPeers := cfg.Trust.Trustors
			peers := make([]peer.ID, len(trustedPeers))

			for i, trustedPeer := range trustedPeers {
				peerID, err := peer.Decode(trustedPeer)
				if err != nil {
					return nil, errors.Wrap(err, "cannot decode peer id")
				}

				peers[i] = peerID
			}

			modules := make(map[string]module.AuditorModule)
			for _, m := range params.Modules {
				modules[m.Type()] = m
			}

			return auditor.NewAuditor(
				auditor.Config{
					PeerID:                  params.Libp2p.ID(),
					TrustManager:            params.TrustManager,
					ResultPublisher:         params.ResultPublisher,
					TaskPublisherSubscriber: params.TaskPublisherSubscriber,
					Modules:                 modules,
					BiddingWait:             cfg.Auditor.BiddingWait,
					ClientRPC:               params.ClientRPC,
				},
			)
		},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot provide auditor")
	}

	// DI: observer.Observer
	err = container.Provide(
		func(db *gorm.DB, resultSubscriber store.Subscriber, trustManager *trust.Manager) (*observer.Observer, error) {
			return observer.NewObserver(db, trustManager, resultSubscriber)
		},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot provide observer")
	}

	return cfg, nil
}

func runRPCServer(ctx context.Context, configPath string, tmpDir string) error {
	container := dig.New()
	// Create a ConsoleWriter output writer that writes to standard output
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout}

	// Create a new logger with the ConsoleWriter output writer
	log := zerolog.New(consoleWriter).With().Str("role", "rpc-server").Caller().Timestamp().Logger()

	cfg, err := setupDependencies(ctx, container, configPath)
	if err != nil {
		return errors.Wrap(err, "cannot setup dependencies")
	}

	if cfg.Auditor.Enabled {
		log.Info().Msg("starting rpc with module")

		err = container.Invoke(
			func(rpcServer *rpcserver.RPCServer) {
				err := rpcServer.Start(ctx, 0, tmpDir)
				if err != nil {
					log.Fatal().Err(err).Msg("cannot start rpc for validation check")
				}
			},
		)
		if err != nil {
			return errors.Wrap(err, "cannot start auditor rpc server")
		}
		return nil
	} else {
		return errors.New("auditor is not enabled")
	}
}

func run(ctx context.Context, configPath string) error {
	container := dig.New()

	cfg, err := setupDependencies(ctx, container, configPath)
	if err != nil {
		return errors.Wrap(err, "cannot setup dependencies")
	}

	anyEnabled := cfg.Dispatcher.Enabled || cfg.Auditor.Enabled || cfg.Observer.Enabled
	log := log3.With().Str("role", "main").Caller().Logger()

	if cfg.Dispatcher.Enabled {
		log.Info().Msg("starting dispatcher")

		//nolint:contextcheck
		err = container.Invoke(
			func(dispatcher *dispatcher.Dispatcher) {
				dispatcher.Start(ctx)
				setupAPI(dispatcher, cfg)
			},
		)
		if err != nil {
			return errors.Wrap(err, "cannot start dispatcher")
		}
	}

	if cfg.Auditor.Enabled {
		log.Info().Msg("starting auditor")

		err = container.Invoke(
			func(auditor *auditor.Auditor) {
				auditor.Start(ctx)
			},
		)
		if err != nil {
			return errors.Wrap(err, "cannot start auditor")
		}
	}

	if cfg.Observer.Enabled {
		log.Info().Msg("starting observer")

		err = container.Invoke(
			func(observer *observer.Observer) {
				observer.Start(ctx)
			},
		)
		if err != nil {
			return errors.Wrap(err, "cannot start observer")
		}
	}

	if !anyEnabled {
		return errors.New("no components enabled")
	}

	<-ctx.Done()
	log.Info().Msg("shutting down")
	return nil
}
