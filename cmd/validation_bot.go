package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
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
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
	"github.com/urfave/cli/v2"
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

func generateNewPeer() (privateStr string, publicStr string, peerStr string, err error) {
	private, public, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return "", "", "", err
	}

	peerID, err := peer.IDFromPublicKey(public)
	if err != nil {
		return "", "", "", err
	}

	privateBytes, err := crypto.MarshalPrivateKey(private)
	if err != nil {
		return "", "", "", err
	}

	privateStr = base64.StdEncoding.EncodeToString(privateBytes)

	publicBytes, err := crypto.MarshalPublicKey(public)
	if err != nil {
		return "", "", "", err
	}
	publicStr = base64.StdEncoding.EncodeToString(publicBytes)
	return privateStr, publicStr, peerID.String(), nil
}

func setConfig(configPath string) error {
	log.Debug().Msg("setting up config default values")
	viper.SetDefault("dispatcher.enabled", true)
	viper.SetDefault("auditor.enabled", true)
	viper.SetDefault("observer.enabled", true)

	viper.SetDefault("observer.database_connection_string", "host=localhost port=5432 user=postgres password=postgres dbname=postgres")
	viper.SetDefault("dispatcher.database_connection_string", "host=localhost port=5432 user=postgres password=postgres dbname=postgres")

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
	viper.SetDefault("observer.retry_interval", time.Minute*1)
	viper.SetDefault("observer.poll_interval", time.Minute*5)

	viper.SetDefault("module.echo.enabled", true)

	viper.SetConfigFile(configPath)
	var newFile bool
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Warn().Str("config_path", configPath).Msg("config file does not exist, creating new one")
		err = os.WriteFile("./config.toml", []byte{}, 0644)
		newFile = true
		if err != nil {
			return errors.Wrap(err, "cannot write defaults to config file")
		}
	}

	log.Debug().Str("config_path", configPath).Msg("reading config file")
	err := viper.ReadInConfig()
	if err != nil {
		return errors.Wrap(err, "cannot read config file")
	}

	if newFile {
		log.Debug().Msg("generating new peers for dispatcher and auditor")
		auditorKey, _, auditorPeer, err := generateNewPeer()
		if err != nil {
			return errors.Wrap(err, "cannot generate auditor key")
		}
		dispatcherKey, _, dispatcherPeer, err := generateNewPeer()
		if err != nil {
			return errors.Wrap(err, "cannot generate dispatcher key")
		}

		viper.Set("auditor.private_key", auditorKey)
		viper.Set("dispatcher.private_key", dispatcherKey)
		viper.Set("auditor.trusted_peers", []string{dispatcherPeer})
		viper.Set("observer.trusted_peers", []string{auditorPeer})
		log.Info().Str("config_path", configPath).Msg("writing defaults to config file")
		err = viper.WriteConfig()
		if err != nil {
			return errors.Wrap(err, "cannot write defaults to created config file")
		}
	}

	envBindingMap := map[string]string{
		"auditor.w3s_token":      "W3S_TOKEN",
		"auditor.private_key":    "AUDITOR_PRIVATE_KEY",
		"dispatcher.private_key": "DISPATCHER_PRIVATE_KEY",
	}

	for key, env := range envBindingMap {
		log.Debug().Str("key", key).Str("env", env).Msg("binding environment variables to config")
		err = viper.BindEnv(key, env)
		if err != nil {
			return errors.Wrap(err, "cannot bind env variable")
		}
	}

	return nil
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
	observerErrorChannel <-chan error) error {
	log.Debug().Msg("subscribing to errors")
	select {
	case err := <-dispatcherErrorChannel:
		return errors.Wrap(err, "dispatcher error")
	case err := <-auditorErrorChannel:
		return errors.Wrap(err, "auditor error")
	case err := <-observerErrorChannel:
		return errors.Wrap(err, "observer error")
	case <-ctx.Done():
		log.Info().Msg("shutting down")
		return nil
	}
}

func run(configPath string) error {
	err := setConfig(configPath)
	if err != nil {
		return err
	}
	api := echo.New()
	api.Logger = lecho.From(log.Logger)
	api.Use(middleware.Recover())
	ctx := context.Background()
	var dispatcherErrorChannel, auditorErrorChannel, observerErrorChannel <-chan error
	var anyEnabled bool
	if viper.GetBool("dispatcher.enabled") {
		anyEnabled = true
		log.Info().Msg("starting dispatcher")
		dispatcher, err := newDispatcher(ctx)
		if err != nil {
			return errors.Wrap(err, "cannot create dispatcher")
		}

		dispatcherErrorChannel = dispatcher.Start(ctx)

		log.Info().Msg("starting dispatcher api")
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
		anyEnabled = true
		log.Info().Msg("starting auditor")
		auditor, err := newAuditor(ctx)
		if err != nil {
			return errors.Wrap(err, "cannot create auditor")
		}

		auditorErrorChannel = auditor.Start(ctx)
	}
	if viper.GetBool("observer.enabled") {
		anyEnabled = true
		log.Info().Msg("starting observer")
		observer, err := newObserver()
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
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	var configPath string
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
						Value:       "./config.toml",
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
					privateStr, publicStr, peerStr, err := generateNewPeer()
					if err != nil {
						return err
					}

					fmt.Println("New peer generated using ed25519, keys are encoded in base64")
					fmt.Println("peer id:     ", peerStr)
					fmt.Println("public key:  ", publicStr)
					fmt.Println("private key: ", privateStr)
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

func newObserver() (*observer.Observer, error) {
	retryInterval := viper.GetDuration("observer.retry_interval")
	pollInterval := viper.GetDuration("observer.poll_interval")
	resultSubscriber := store.NewW3StoreSubscriber(retryInterval, pollInterval)
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

	token := viper.GetString("auditor.w3s_token")
	if token == "" {
		return nil, errors.New("auditor.w3s_token is empty")
	}
	resultPublisher, err := store.NewW3StorePublisher(
		token,
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

func (m *MockTaskCreator) List(ctx context.Context) ([]task.Definition, error) {
	args := m.Called(ctx)
	return args.Get(0).([]task.Definition), args.Error(1)
}
