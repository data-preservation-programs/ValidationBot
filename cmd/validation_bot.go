package main

import (
	"context"

	"validation-bot/auditor"
	"validation-bot/store"

	"validation-bot/dispatcher"

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

func main() {
	ctx := context.Background()
	api := echo.New()
	api.Logger = lecho.From(log.Logger)
	api.Use(middleware.Recover())
	if viper.GetBool("dispatcher.enabled") {
		dispatcherGroup, err := startDispatcher(ctx)
		if err != nil {
			log.Error().Err(err).Msg("cannot start dispatcherGroup")
			return
		}

		api.POST("/dispatcherGroup/task", func(c echo.Context) error {
			var definition task.Definition
			err := c.Bind(&definition)
			if err != nil {
				return err
			}

			err = dispatcherGroup.Create(c.Request().Context(), &definition)
			if err != nil {
				return err
			}

			return c.NoContent(200)
		})

		api.DELETE("/dispatcherGroup/task/:id", func(c echo.Context) error {
			id := c.Param("id")
			parsedId, err := uuid.Parse(id)
			if err != nil {
				return c.String(400, "invalid id")
			}

			err = dispatcherGroup.Remove(c.Request().Context(), parsedId)
			if err != nil {
				return errors.Wrap(err, "cannot delete task definition")
			}

			return c.String(200, "ok")
		})

		api.GET("/dispatcherGroup/task", func(c echo.Context) error {
			definitions, err := dispatcherGroup.List(c.Request().Context())
			if err != nil {
				return errors.Wrap(err, "cannot list task definitions")
			}

			return c.JSON(200, definitions)
		})
	}
	if viper.GetBool("auditor.enabled") {
		err := startAuditor(ctx)
		if err != nil {
			log.Error().Err(err).Msg("cannot start auditor")
			return
		}
	}
}

func startAuditor(ctx context.Context) error {
	pubsubConfig, err := task.NewPubsubConfig(
		viper.GetString("auditor.private_key"),
		viper.GetString("auditor.listen_addr"),
		viper.GetString("auditor.topic_name"))
	if err != nil {
		return errors.Wrap(err, "cannot create pubsub config")
	}

	taskSubscriber, err := task.NewLibp2pTaskSubscriber(ctx, *pubsubConfig)
	if err != nil {
		return errors.Wrap(err, "cannot create task subscriber")
	}

	// TODO provider implementation
	var resultPublisher store.ResultPublisher

	trustedPeers := viper.GetStringSlice("auditor.trusted_peers")
	peers := make([]peer.ID, len(trustedPeers))
	for i, trustedPeer := range trustedPeers {
		peerId, err := peer.Decode(trustedPeer)
		if err != nil {
			return errors.Wrap(err, "cannot decode peer id")
		}
		peers[i] = peerId
	}

	auditor, err := auditor.NewAuditor(auditor.Config{
		ResultPublisher: resultPublisher,
		TaskSubscriber:  taskSubscriber,
		TrustedPeers:    peers,
	})
	if err != nil {
		return errors.Wrap(err, "cannot create auditor")
	}

	go func() {
		err := auditor.Start(ctx)
		if err != nil {
			log.Error().Err(err).Msg("auditor stopped")
		}
	}()

	return nil
}

func startDispatcher(ctx context.Context) (*dispatcher.Group, error) {
	connectionString := viper.GetString("dispatcher.database_connection_string")
	db, err := gorm.Open(postgres.Open(connectionString), &gorm.Config{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot open database connection")
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

	dispatcherConfig := dispatcher.Config{
		Db:            db.WithContext(ctx),
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
