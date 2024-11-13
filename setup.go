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
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"strings"

	"connectrpc.com/authn"
	"connectrpc.com/connect"
	"connectrpc.com/otelconnect"
	v1Connect "github.com/choopm/knxrpc/knx/groupaddress/v1/v1connect"
	"github.com/choopm/knxrpc/web"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/vapourismo/knx-go/knx/util"
	"github.com/ziflex/lecho/v3"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
)

// setup sets up initial KNX state
func (s *Server) setup() error {
	if err := s.setupKNXLogger(); err != nil {
		return err
	}

	if err := s.setupOpenTelemetry(); err != nil {
		return err
	}

	if err := s.setupRPCHandler(); err != nil {
		return err
	}

	if err := s.setupWebserver(); err != nil {
		return err
	}

	return nil
}

// setupKNXLogger sets up the logger by wrapping s.log
func (s *Server) setupKNXLogger() error {
	util.Logger = &knxLogHandler{
		log:    s.log,
		cancel: s.cancel,
	}
	return nil
}

// setupOpenTelemetry configures the OpenTelemetry pipeline or error
func (s *Server) setupOpenTelemetry() (err error) {
	if !s.config.RPC.Webserver.Metrics.Enabled {
		// early return if no metrics are configured
		return nil
	}

	s.metricExporter, err = prometheus.New()
	if err != nil {
		return err
	}
	s.meterProvider = metric.NewMeterProvider(metric.WithReader(s.metricExporter))

	return nil
}

// setupRPCHandler initializes s.Handler
func (s *Server) setupRPCHandler() error {
	opts := []connect.HandlerOption{}

	// otel metrics interceptor
	if s.config.RPC.Webserver.Metrics.Enabled {
		otelInterceptor, err := otelconnect.NewInterceptor(
			otelconnect.WithMeterProvider(s.meterProvider),
			otelconnect.WithoutServerPeerAttributes(),
		)
		if err != nil {
			return err
		}
		opts = append(opts, connect.WithInterceptors(otelInterceptor))
	}

	// register RPCs at ServeMux
	mux := http.NewServeMux()
	mux.Handle(v1Connect.NewGroupAddressServiceHandler(s, opts...))
	s.Handler = mux

	// early return if no authentication is required
	if !s.config.RPC.Auth.Enabled {
		return nil
	}

	// wrap the mux into an authMiddleware
	authMiddleware := authn.NewMiddleware(s.authenticateRPC, opts...)
	s.Handler = authMiddleware.Wrap(mux)

	return nil
}

// setupWebserver sets up echo webserver or error
func (s *Server) setupWebserver() error {
	if !s.config.RPC.Webserver.Enabled {
		return nil
	}

	// create echo
	s.e = echo.New()
	s.e.HideBanner = true
	s.e.HidePort = true
	s.e.Use(middleware.Recover())

	if s.config.RPC.Webserver.LogRequests {
		s.e.Logger = lecho.From(*s.log)
		s.e.Use(middleware.RequestID())
		s.e.Use(middleware.Logger())
	}

	// bind servemux
	s.e.Group("/knx.*").Use(echo.WrapMiddleware(func(next http.Handler) http.Handler {
		return s.Handler
	}))

	// bind metrics
	if s.config.RPC.Webserver.Metrics.Enabled {
		middlewares := []echo.MiddlewareFunc{}
		if s.config.RPC.Webserver.Metrics.Auth.Enabled {
			middlewares = append(middlewares, middleware.KeyAuthWithConfig(
				middleware.KeyAuthConfig{
					KeyLookup:  "header:" + s.config.RPC.Webserver.Metrics.Auth.Header,
					AuthScheme: s.config.RPC.Webserver.Metrics.Auth.Scheme,
					Validator: func(auth string, c echo.Context) (bool, error) {
						err := s.authenticateStaticSecretKey(auth)
						if err != nil {
							return false, err
						}

						return true, nil
					},
				},
			))
		}
		middlewares = append(middlewares, echo.WrapMiddleware(func(next http.Handler) http.Handler {
			return promhttp.Handler()
		}))
		s.e.Group(s.config.RPC.Webserver.Metrics.Path, middlewares...)
	}

	// early return if swagger is disabled
	if !s.config.RPC.Webserver.Swagger.Enabled {
		return nil
	}

	// build swagger path
	swagPath, err := url.JoinPath("/", s.config.RPC.Webserver.Swagger.Path)
	if err != nil {
		return fmt.Errorf("unable to build swagger.path: %s", err)
	}
	swagPath = strings.TrimSuffix(swagPath, "/")

	// get the subdirectory content
	swagFS, err := fs.Sub(web.SwaggerFS, "swagger")
	if err != nil {
		return err
	}

	// bind swagger fs fileserver
	swagHandler := http.FileServer(http.FS(swagFS))
	s.e.Group(swagPath).Use(echo.WrapMiddleware(func(next http.Handler) http.Handler {
		return http.StripPrefix(swagPath, swagHandler)
	}))

	// early return if swagger.redirect is disabled
	if !s.config.RPC.Webserver.Swagger.RootRedirect {
		return nil
	}

	// bind root redirect
	redirect := func(c echo.Context) error {
		return c.Redirect(http.StatusTemporaryRedirect, swagPath+"/")
	}
	s.e.GET("", redirect)
	s.e.GET("/", redirect)

	return nil
}
