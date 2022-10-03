package main

import (
	"context"
	"os"
	"time"

	"validation-bot/module"
	echo_module "validation-bot/module/echo"
	"validation-bot/role/dispatcher"

	"validation-bot/role/auditor"
	"validation-bot/role/observer"

	"validation-bot/store"

	"validation-bot/task"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"github.com/ziflex/lecho/v3"
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

func setDefault(configPath string) {
	viper.SetDefault("dispatcher.enabled", false)
	viper.SetDefault("auditor.enabled", false)
	viper.SetDefault("observer.enabled", false)

	viper.SetDefault("observer.database_connection_string", "database_connection_string")
	viper.SetDefault("dispatcher.database_connection_string", "database_connection_string")

	viper.SetDefault("observer.trusted_peers", []string{})
	viper.SetDefault("auditor.trusted_peers", []string{})

	viper.SetDefault("auditor.private_key", "")
	viper.SetDefault("dispatcher.private_key", "")

	viper.SetDefault("auditor.listen_addr", "/ip4/0.0.0.0/tcp/7999")
	viper.SetDefault("dispatcher.listen_addr", "/ip4/0.0.0.0/tcp/7998")

	viper.SetDefault("auditor.topic_name", "/filecoin/validation_bot/dev")
	viper.SetDefault("dispatcher.topic_name", "/filecoin/validation_bot/dev")

	viper.SetDefault("auditor.w3s_token", "")
	viper.SetDefault("dispatcher.check_interval", time.Minute*5)

	viper.SetDefault("module.echo.enabled", true)

	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath("$HOME/.validation_bot")
	viper.AddConfigPath(".")
	viper.SetConfigFile(configPath)
	err := viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Warn().Msg("Config file not found, using default values")
		} else {
			log.Error().Err(err).Msg("Cannot read config file")
		}
	}

	err = viper.BindEnv("auditor.w3s_token", "W3S_TOKEN")
	if err != nil {
		panic(err)
	}
	err = viper.BindEnv("auditor.private_key", "AUDITOR_PRIVATE_KEY")
	if err != nil {
		panic(err)
	}
	err = viper.BindEnv("dispatcher.private_key", "DISPATCHER_PRIVATE_KEY")
	if err != nil {
		panic(err)
	}
}

func deleteTaskHandler(c echo.Context, dispatcher taskRemover) error {
	id := c.Param("id")
	parsedId, err := uuid.Parse(id)
	if err != nil {
		return c.String(400, "invalid id")
	}

	err = dispatcher.Remove(c.Request().Context(), parsedId)
	if err != nil {
		return errors.Wrap(err, "cannot delete task definition")
	}

	return c.NoContent(200)
}

func postTaskHandler(c echo.Context, dispatcher taskCreator) error {
	var definition task.Definition
	err := c.Bind(&definition)
	if err != nil {
		return err
	}

	err = dispatcher.Create(c.Request().Context(), &definition)
	if err != nil {
		return err
	}

	return c.JSON(200, definition)
}

func listTasksHandler(c echo.Context, dispatcher taskLister) error {
	definitions, err := dispatcher.List(c.Request().Context())
	if err != nil {
		return errors.Wrap(err, "cannot list task definitions")
	}

	return c.JSON(200, definitions)
}

func subscribeToErrors(ctx context.Context,
	dispatcherErrorChannel <-chan error,
	auditorErrorChannel <-chan error,
	observerErrorChannel <-chan error) {
	select {
	case err := <-dispatcherErrorChannel:
		log.Error().Err(err).Msg("dispatcher error")
		os.Exit(1)
	case err := <-auditorErrorChannel:
		log.Error().Err(err).Msg("auditor error")
		os.Exit(1)
	case err := <-observerErrorChannel:
		log.Error().Err(err).Msg("observer error")
		os.Exit(1)
	case <-ctx.Done():
		log.Info().Msg("shutting down")
	}
}

func main() {
	ctx := context.Background()
	args := os.Args[1:]
	configPath := ""
	if len(args) > 0 {
		configPath = args[0]
	}
	setDefault(configPath)
	api := echo.New()
	api.Logger = lecho.From(log.Logger)
	api.Use(middleware.Recover())
	var dispatcherErrorChannel, auditorErrorChannel, observerErrorChannel <-chan error
	if viper.GetBool("dispatcher.enabled") {
		dispatcher, err := newDispatcher(ctx)
		if err != nil {
			log.Error().Err(err).Msg("cannot create dispatcher")
			return
		}

		dispatcherErrorChannel = dispatcher.Start(ctx)

		api.POST(createRoute, func(c echo.Context) error {
			return postTaskHandler(c, dispatcher)
		})

		api.DELETE(deleteRoute, func(c echo.Context) error {
			return deleteTaskHandler(c, dispatcher)
		})

		api.GET(listRoute, func(c echo.Context) error {
			return listTasksHandler(c, dispatcher)
		})
	}

	if viper.GetBool("auditor.enabled") {
		auditor, err := newAuditor(ctx)
		if err != nil {
			log.Error().Err(err).Msg("cannot create auditor")
			return
		}

		auditorErrorChannel = auditor.Start(ctx)
	}
	if viper.GetBool("observer.enabled") {
		observer, err := newObserver()
		if err != nil {
			log.Error().Err(err).Msg("cannot create observer")
			return
		}

		observerErrorChannel = observer.Start(ctx)
	}

	subscribeToErrors(ctx, dispatcherErrorChannel, auditorErrorChannel, observerErrorChannel)
}

func newObserver() (*observer.Observer, error) {
	retryInterval := viper.GetInt64("observer.retry_interval_second")
	resultSubscriber := store.NewW3StoreSubscriber(time.Duration(retryInterval) * time.Second)
	connectionString := viper.GetString("observer.database_connection_string")
	db, err := gorm.Open(postgres.Open(connectionString), &gorm.Config{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot open database connection")
	}

	trustedPeers := viper.GetStringSlice("observer.trusted_peers")
	peers := make([]peer.ID, len(trustedPeers))
	for i, trustedPeer := range trustedPeers {
		peerId, err := peer.Decode(trustedPeer)
		if err != nil {
			return nil, errors.Wrap(err, "cannot decode peer id")
		}
		peers[i] = peerId
	}

	var modules []module.Module
	if viper.GetBool("module.echo.enabled") {
		echoModule := echo_module.Echo{}
		modules = append(modules, echoModule)
	}

	observer, err := observer.NewObserver(db, resultSubscriber, peers, modules)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create observer")
	}

	return observer, nil
}

func newAuditor(ctx context.Context) (*auditor.Auditor, error) {
	pubsubConfig, err := task.NewPubsubConfig(
		viper.GetString("auditor.private_key"),
		viper.GetString("auditor.listen_addr"),
		viper.GetString("auditor.topic_name"))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create pubsub config")
	}

	taskSubscriber, err := task.NewLibp2pTaskSubscriber(ctx, *pubsubConfig)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create task subscriber")
	}

	resultPublisher, err := store.NewW3StorePublisher(
		viper.GetString("auditor.w3s_token"),
		viper.GetString("auditor.private_key"))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create result publisher")
	}

	trustedPeers := viper.GetStringSlice("auditor.trusted_peers")
	peers := make([]peer.ID, len(trustedPeers))
	for i, trustedPeer := range trustedPeers {
		peerId, err := peer.Decode(trustedPeer)
		if err != nil {
			return nil, errors.Wrap(err, "cannot decode peer id")
		}
		peers[i] = peerId
	}

	auditor, err := auditor.NewAuditor(auditor.Config{
		ResultPublisher: resultPublisher,
		TaskSubscriber:  taskSubscriber,
		TrustedPeers:    peers,
	})
	if err != nil {
		return nil, errors.Wrap(err, "cannot create auditor")
	}

	return auditor, nil
}

func newDispatcher(ctx context.Context) (*dispatcher.Dispatcher, error) {
	connectionString := viper.GetString("dispatcher.database_connection_string")
	db, err := gorm.Open(postgres.Open(connectionString), &gorm.Config{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot open database connection")
	}

	err = db.WithContext(ctx).AutoMigrate(&task.Definition{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot migrate task definitions")
	}

	pubsubConfig, err := task.NewPubsubConfig(
		viper.GetString("dispatcher.private_key"),
		viper.GetString("dispatcher.listen_addr"),
		viper.GetString("dispatcher.topic_name"))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create pubsub config")
	}

	taskPublisher, err := task.NewLibp2pTaskPublisher(ctx, *pubsubConfig)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create task publisher")
	}

	var modules []module.Module
	if viper.GetBool("module.echo.enabled") {
		echoModule := echo_module.Echo{}
		modules = append(modules, echoModule)
	}

	dispatcherConfig := dispatcher.Config{
		Db:            db,
		TaskPublisher: taskPublisher,
		CheckInterval: viper.GetDuration("dispatcher.check_interval"),
		Modules:       modules,
	}

	dispatcher, err := dispatcher.NewDispatcher(dispatcherConfig)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create dispatcher")
	}
	return dispatcher, nil
}
