/*
Copyright 2024 Christoph Hoopmann

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"go.uber.org/fx"

	"github.com/choopm/knxrpc"
	v1 "github.com/choopm/knxrpc/knx/groupaddress/v1"
	"github.com/choopm/stdfx"
	"github.com/choopm/stdfx/configfx"
	"github.com/choopm/stdfx/loggingfx/zerologfx"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// version is provided by `-ldflags "-X main.version=1.0.0"`
var version string = "unknown"

func main() {
	fx.New(
		// logging
		zerologfx.Module,
		fx.WithLogger(zerologfx.ToFx),
		fx.Decorate(zerologfx.Decorator[knxrpc.Config]),

		// viper configuration
		fx.Provide(stdfx.ConfigFile[knxrpc.Config]("knxrpc")),

		// cobra commands
		fx.Provide(
			stdfx.AutoRegister(stdfx.VersionCommand(version)),
			stdfx.AutoRegister(stdfx.ConfigCommand[knxrpc.Config]),
			stdfx.AutoRegister(serverCommand),
			stdfx.AutoRegister(subscribeCommand),
			stdfx.AutoRegister(publishCommand),
			stdfx.AutoCommand, // add registered commands to root
		),

		// app start
		fx.Invoke(stdfx.Unprivileged), // abort when being run as root
		fx.Invoke(stdfx.Commander),    // run root cobra command
	).Run()
}

// serverCommand returns a *cobra.Command to start the server from a ConfigProvider
func serverCommand(
	configProvider configfx.Provider[knxrpc.Config],
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "server - starts knxrpc",
		RunE: func(cmd *cobra.Command, args []string) error {
			// fetch the config
			cfg, err := configProvider.Config()
			if err != nil {
				return err
			}

			// rebuild logger and make it global
			logger, err := zerologfx.New(cfg.Log)
			if err != nil {
				return err
			}
			log.Logger = *logger

			// create knxrpc instance
			server, err := knxrpc.New(cfg, logger)
			if err != nil {
				return err
			}

			// start knxrpc using context
			return server.Start(cmd.Context())
		},
	}

	return cmd
}

// subscribeCommand returns a *cobra.Command to start a subscriber from a ConfigProvider
func subscribeCommand(
	configProvider configfx.Provider[knxrpc.Config],
) *cobra.Command {
	fls := pflag.NewFlagSet("subscribe", pflag.ContinueOnError)
	eventFilter := fls.String("event", "",
		"optional filter for events, oneof: read|write|response")

	cmd := &cobra.Command{
		Use:   "subscribe [1/2/3]...",
		Short: "subscribe - connects to knxrpc and lists messages",
		Long:  "filtered for group addresses provided as optional arguments",
		RunE: func(cmd *cobra.Command, args []string) error {
			// parse event filter
			ev := v1.Event_EVENT_UNSPECIFIED
			switch *eventFilter {
			case "":
				break
			case "read":
				ev = v1.Event_EVENT_READ
			case "write":
				ev = v1.Event_EVENT_WRITE
			case "response":
				ev = v1.Event_EVENT_RESPONSE
			default:
				return fmt.Errorf("unsupported event filter: %s", *eventFilter)
			}

			// fetch the config
			cfg, err := configProvider.Config()
			if err != nil {
				return err
			}

			// rebuild logger and make it global
			logger, err := zerologfx.New(cfg.Log)
			if err != nil {
				return err
			}
			log.Logger = *logger

			logger.Info().
				Strs("group-address-filter", args).
				Str("event-filter", *eventFilter).
				Str("host", cfg.Client.Host).
				Int("port", cfg.Client.Port).
				Bool("auth", cfg.Client.Auth.Enabled).
				Msg("waiting for messages")

			// create the client instance
			client, err := knxrpc.NewClient(cfg.Client)
			if err != nil {
				return err
			}

			// connect stream using group addresses and events from args
			stream, err := client.Subscribe(cmd.Context(),
				connect.NewRequest(&v1.SubscribeRequest{
					GroupAddresses: args,
					Event:          ev,
				}))
			if err != nil {
				return err
			}
			// close stream on context.Done()
			context.AfterFunc(cmd.Context(), func() { stream.Close() }) // nolint:errcheck

			// start receiver loop
			for stream.Receive() {
				res := stream.Msg()
				logger.Info().
					Str("group-address", res.GroupAddress).
					Str("physical-address", res.PhysicalAddress).
					Str("event", res.Event.String()).
					Bytes("data", res.Data).
					Msg("received message")
			}
			if err := stream.Err(); err != nil &&
				!errors.Is(err, context.Canceled) {
				return err
			}

			return nil
		},
	}
	cmd.Flags().AddFlagSet(fls)

	return cmd
}

// publishCommand returns a *cobra.Command to publish a message from a ConfigProvider
func publishCommand(
	configProvider configfx.Provider[knxrpc.Config],
) *cobra.Command {
	fls := pflag.NewFlagSet("publish", pflag.ContinueOnError)
	eventType := fls.String("event", "",
		"event to send, oneof: read|write|response")
	physicalAddress := fls.String("from", "",
		"optionial physical address, e.g.: 1.2.3")

	cmd := &cobra.Command{
		Use:   "publish <1/2/3> [data]",
		Short: "publish - connects to knxrpc and sends an event using data",
		Long:  "for the group address provided",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// parse event type
			ev := v1.Event_EVENT_UNSPECIFIED
			switch *eventType {
			case "":
				if len(args) > 1 {
					// send write event by default if data was given
					ev = v1.Event_EVENT_WRITE
				} else {
					ev = v1.Event_EVENT_READ
				}
			case "read":
				ev = v1.Event_EVENT_READ
			case "write":
				ev = v1.Event_EVENT_WRITE
			case "response":
				ev = v1.Event_EVENT_RESPONSE
			default:
				return fmt.Errorf("unsupported event type: %s", *eventType)
			}

			// parse data if any
			var dataBytes []byte
			var err error
			if len(args) > 1 {
				dataBytes, err = hex.DecodeString(args[1])
				if err != nil {
					return fmt.Errorf("unable to decode hex data: %v", err)
				}
			}

			// fetch the config
			cfg, err := configProvider.Config()
			if err != nil {
				return err
			}

			// rebuild logger and make it global
			logger, err := zerologfx.New(cfg.Log)
			if err != nil {
				return err
			}
			log.Logger = *logger

			logger.Trace().
				Str("group-address", args[0]).
				Str("event-type", ev.String()).
				Str("host", cfg.Client.Host).
				Int("port", cfg.Client.Port).
				Bool("auth", cfg.Client.Auth.Enabled).
				Msg("sending message")

			// create the client instance
			client, err := knxrpc.NewClient(cfg.Client)
			if err != nil {
				return err
			}

			// publish the messsage event
			_, err = client.Publish(cmd.Context(),
				connect.NewRequest(&v1.PublishRequest{
					GroupAddress:    args[0],
					PhysicalAddress: *physicalAddress,
					Data:            dataBytes,
					Event:           ev,
				}))
			if err != nil {
				return err
			}

			logger.Info().
				Str("group-address", args[0]).
				Str("data", hex.EncodeToString(dataBytes)).
				Str("event-type", ev.String()).
				Msg("message sent")

			return nil
		},
	}
	cmd.Flags().AddFlagSet(fls)

	return cmd
}
