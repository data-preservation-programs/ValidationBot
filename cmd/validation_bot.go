package main

import (
	"fmt"
	"os"
	"time"

	"validation-bot/role/trust"

	"validation-bot/role"

	"validation-bot/rpcv"
	"validation-bot/store"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log3 "github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

//nolint:funlen,forbidigo,dupl
func main() {
	var configPath string
	var privateKey string
	var w3sToken string
	var peerIDSlice cli.StringSlice
	var peerIDStr string
	var operationType string
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
				Name:  "validation-rpc",
				Usage: "start running the rpcv server",
				Action: func(c *cli.Context) error {
					addr, err := rpcv.NewRPCServer()
					if err != nil {
						return errors.Wrap(err, "failed to start rpc server")
					}

					fmt.Println(addr.Port)

					<-c.Done()
					log.Info().Msg("shutting down")
					return nil
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
				Name:  "get-peer-id-from-private-key",
				Usage: "get peer id from private key",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "key",
						Aliases:     []string{"k"},
						Usage:       "private key",
						Destination: &privateKey,
						Required:    true,
					},
				},
				Action: func(c *cli.Context) error {
					trustorPeerID, err := role.GetPeerIDFromPrivateKeyStr(privateKey)
					if err != nil {
						return errors.Wrap(err, "cannot get peer id from private key")
					}

					fmt.Println("peer id: ", trustorPeerID.String())
					return nil
				},
			},
			{
				Name:  "modify-trusted-peer",
				Usage: "create/revoke/reset trusted peers and publish to the network via W3Sname",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "type",
						Usage:       "create/revoke/reset",
						Destination: &operationType,
						Required:    true,
					},
					&cli.StringFlag{
						Name:        "trustor-key",
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
					&cli.StringSliceFlag{
						Name:        "peer",
						Aliases:     []string{"p"},
						Usage:       "trustee peerID(s) string to operate on",
						Destination: &peerIDSlice,
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

					peerIDs := make([]peer.ID, 0)
					for _, peerIDStr := range peerIDSlice.Value() {
						peerID, err := peer.Decode(peerIDStr)
						if err != nil {
							return errors.Wrap(err, "cannot decode peer id")
						}

						peerIDs = append(peerIDs, peerID)
					}

					trustorPeerID, err := role.GetPeerIDFromPrivateKeyStr(privateKey)
					if err != nil {
						return errors.Wrap(err, "cannot get peer id from private key")
					}

					err = trust.ModifyPeers(
						c.Context,
						publisher,
						subscriber,
						operationType,
						trustorPeerID,
						peerIDs,
						time.Second,
					)
					if err != nil {
						return errors.Wrap(err, "cannot publish record to trust new peer")
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
					for _, peerStr := range peers {
						fmt.Printf("%s\n", peerStr)
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
