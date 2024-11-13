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

package knxrpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	v1Connect "github.com/choopm/knxrpc/knx/groupaddress/v1/v1connect"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/vapourismo/knx-go/knx"
	"github.com/vapourismo/knx-go/knx/cemi"
	"go.opentelemetry.io/otel/sdk/metric"
	"golang.org/x/sync/errgroup"
)

// Server implements RPCs using a http.Handler.
type Server struct {
	http.Handler
	v1Connect.UnimplementedGroupAddressServiceHandler

	// holds Config during runtime
	config *Config

	// log is used to log things
	log *zerolog.Logger

	// ctx stores the ctx of this server so that RPCi can watch it
	ctx    context.Context
	cancel context.CancelFunc

	// tunnel stores the connected KNX tunnel
	tunnel knx.GroupTunnel

	// e stores the echo instance if any
	e *echo.Echo

	// metricExporter stores the otel exporter
	metricExporter metric.Reader

	// meterProvider stores the OpenTelemetry MeterProvider
	meterProvider *metric.MeterProvider

	// --- RPC and open streams related down below ---

	// subscribers stores all group addresses to connected streams
	subscribers map[cemi.GroupAddr][]*subscriber
	// m_subscribers synchronizes access to subscribers
	m_subscribers sync.Mutex

	// sniffers stores subscribers which receive all group addresses (no filtering)
	sniffers []*subscriber
	// m_sniffers synchronizes access to sniffers
	m_sniffers sync.Mutex
}

// New returns a new *KNXConnect or error
// Use [Start] to connect the KNX tunnel and start dispatching messages.
func New(config *Config, logger *zerolog.Logger) (*Server, error) {
	// validate config
	if config == nil {
		return nil, errors.New("missing config")
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config: %s", err)
	}

	// init logger if missing
	if logger == nil {
		logger = &log.Logger
	}

	s := &Server{
		config:      config,
		log:         logger,
		subscribers: map[cemi.GroupAddr][]*subscriber{},
		sniffers:    []*subscriber{},
	}

	return s, nil
}

// Start will connect to the KNX bus and start message handling or error.
// You may cancel ctx any time to close the tunnel and stop message handling.
func (s *Server) Start(ctx context.Context) error {
	ctx, s.cancel = context.WithCancel(ctx)
	defer s.cancel()

	g, ctx := errgroup.WithContext(ctx)
	s.ctx = ctx

	s.log.Trace().
		Msg("knx knxrpc setup")

	if err := s.setup(); err != nil {
		return err
	}

	s.log.Trace().
		Msg("knx knxrpc connecting")

	// connect it
	if err := s.connectTunnel(); err != nil {
		return err
	}
	defer s.tunnel.Close()
	// bind closer to ctx
	context.AfterFunc(ctx, func() {
		s.tunnel.Close() // nolint:errcheck
	})

	// start webserver
	g.Go(func() error {
		// webserver is not enabled
		if !s.config.RPC.Webserver.Enabled {
			return nil
		}

		// shutdown hook, register before Start()
		context.AfterFunc(ctx, func() {
			err := s.e.Shutdown(ctx)
			if err != nil {
				s.e.Close() // nolint:errcheck
			}
		})

		// print info after started
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
				break
			}

			ev := s.log.Info().
				Str("hostport", s.e.ListenerAddr().String())
			if s.config.RPC.Webserver.Swagger.Enabled {
				hostport := s.e.ListenerAddr().String()
				if strings.Contains(hostport, "[::]") ||
					strings.Contains(hostport, "0.0.0.0") {
					hostport = fmt.Sprintf("localhost:%d", s.config.Client.Port)
				}
				ev = ev.Str("swagger", fmt.Sprintf("http://%s%s/",
					hostport,
					filepath.Join("/", s.config.RPC.Webserver.Swagger.Path),
				))
			}
			ev.Msg("knxrpc listening on")

			return nil
		})

		err := s.e.Start(net.JoinHostPort(
			s.config.RPC.Webserver.Host,
			strconv.Itoa(s.config.RPC.Webserver.Port),
		))
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		return nil
	})

	// start bus reader
	g.Go(func() error {
		return s.busMessageReader(ctx)
	})

	s.log.Trace().
		Msg("knxrpc started")

	// block for all to finish
	err := g.Wait()
	if err != nil {
		return err
	}

	// cleanup
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if s.meterProvider != nil {
		_ = s.meterProvider.Shutdown(ctx)
	}
	if s.metricExporter != nil {
		_ = s.metricExporter.Shutdown(ctx)
	}

	s.log.Trace().
		Msg("knxrpc stopped")

	return nil
}
