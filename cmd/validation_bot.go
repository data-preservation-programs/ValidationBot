package main

import (
	"context"
	"encoding/base64"

	"validation-bot/dispatcher"

	"validation-bot/task"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"github.com/ziflex/lecho/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	ctx := context.Background()
	api := echo.New()
	api.Logger = lecho.From(log.Logger)
	api.Use(middleware.Recover())
	if viper.GetBool("dispatcher.enabled") {
		dispatcher, err := startDispatcher(ctx)
		if err != nil {
			log.Error().Err(err).Msg("cannot start dispatcher")
		}

		api.POST("/dispatcher/task", func(c echo.Context) error {
			definition := new(task.Definition)
			if err := c.Bind(definition); err != nil {
				log.Warn().Err(err).Msg("cannot bind task definition")
				return c.String(400, "cannot parse task definition"+err.Error())
			}
			err = dispatcher.Create(c.Request().Context(), definition)
			if err != nil {
				return errors.Wrap(err, "cannot create task definition")
			}

			type Response struct {
				ID uuid.UUID `json:"id"`
			}

			return c.JSON(200, Response{ID: definition.ID})
		})

		api.DELETE("/dispatcher/task/:id", func(c echo.Context) error {
			id := c.Param("id")
			parsedId, err := uuid.Parse(id)
			if err != nil {
				return c.String(400, "invalid id")
			}

			err = dispatcher.Remove(c.Request().Context(), parsedId)
			if err != nil {
				return errors.Wrap(err, "cannot delete task definition")
			}

			return c.String(200, "ok")
		})

		api.GET("/dispatcher/task", func(c echo.Context) error {
			definitions, err := dispatcher.List(c.Request().Context())
			if err != nil {
				return errors.Wrap(err, "cannot list task definitions")
			}

			return c.JSON(200, definitions)
		})
	}
}

func startDispatcher(ctx context.Context) (*dispatcher.Group, error) {
	connectionString := viper.GetString("database.connection_string")
	db, err := gorm.Open(postgres.Open(connectionString), &gorm.Config{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot connect to the database")
	}

	privateKeyStr := viper.GetString("dispatcher.private_key")
	privateKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyStr)
	if err != nil {
		return nil, errors.Wrap(err, "cannot decode private key")
	}

	privateKey, err := crypto.UnmarshalPrivateKey(privateKeyBytes)
	if err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal private key")
	}

	peerID, err := peer.IDFromPrivateKey(privateKey)

	pubsubConfig := task.PubsubConfig{
		PrivateKey: privateKey,
		PeerID:     peerID,
		ListenAddr: viper.GetString("dispatcher.listen_addr"),
		TopicName:  viper.GetString("dispatcher.topic_name"),
	}

	taskPublisher, err := task.NewLibp2pTaskPublisher(ctx, pubsubConfig)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create task publisher")
	}

	dispatcherConfig := dispatcher.Config{
		Db:            db,
		TaskPublisher: taskPublisher,
	}

	dispatcherGroup, err := dispatcher.NewDispatcherGroup(ctx, dispatcherConfig)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create dispatcher group")
	}

	err = dispatcherGroup.Start(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "cannot start dispatcher group")
	}

	return dispatcherGroup, nil
}
