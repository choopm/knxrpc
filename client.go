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
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"connectrpc.com/connect"
	"github.com/choopm/knxrpc/knx/groupaddress/v1/v1connect"
)

// NewClient returns a fresh GroupAddressServiceClient from config
func NewClient(config ClientConfig, opts ...connect.ClientOption) (v1connect.GroupAddressServiceClient, error) {
	if config.Auth.Enabled {
		opts = append(opts, connect.WithInterceptors(
			NewAuthInterceptor(config.Auth),
		))
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if config.InsecureTLS {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.InsecureSkipVerify = config.InsecureTLS
	}
	hclient := &http.Client{
		Transport: transport,
	}

	scheme := "http://"
	if config.UseTLS {
		scheme = "https://"
	}

	client := v1connect.NewGroupAddressServiceClient(
		hclient,
		fmt.Sprintf("%s%s", scheme, net.JoinHostPort(
			config.Host,
			strconv.Itoa(config.Port))),
		opts...,
	)

	return client, nil
}

// NewAuthInterceptor returns a [headerInterceptor] to add authentication
func NewAuthInterceptor(config AuthConfig) *headerInterceptor {
	return &headerInterceptor{
		name:  config.Header,
		value: config.Scheme + " " + config.SecretKey,
	}
}

// headerInterceptor is an connect.Interceptor implementation
// that wraps RPCs adding a header and its overwriting its value.
type headerInterceptor struct {
	name  string
	value string
}

// WrapUnary implements [Interceptor] by applying the interceptor function.
func (s *headerInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if req.Spec().IsClient {
			req.Header().Set(s.name, s.value)
		}
		return next(ctx, req)
	}
}

// WrapStreamingClient implements [Interceptor] by applying the interceptor function.
func (s *headerInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		if !spec.IsClient {
			return next(ctx, spec)
		}
		conn := next(ctx, spec)
		conn.RequestHeader().Set(s.name, s.value)
		return conn
	}
}

// WrapStreamingHandler implements [Interceptor] with a no-op.
func (s *headerInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}
