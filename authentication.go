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
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"connectrpc.com/authn"
	"connectrpc.com/connect"
)

var (
	ErrInvalidAuthCredentials = errors.New("invalid auth credentials")
)

// AuthConfig holds the auth configuration
type AuthConfig struct {
	// Enabled whether to require and check authentication
	Enabled bool `mapstructure:"enabled" default:"false"`

	// Header is the header to fetch the key, required if [Enabled]
	Header string `mapstructure:"header" default:"Authorization"`

	// Scheme defines the auth scheme which is stripped from the header value
	Scheme string `mapstructure:"scheme" default:"Bearer"`

	// SecretKey is the key to compare the Header value with, required if [Enabled]
	SecretKey string `mapstructure:"secretKey" default:""`
}

// Validate validates the AuthConfig
func (c *AuthConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	if len(c.Header) == 0 {
		return fmt.Errorf("missing server.auth.header")
	}
	if len(c.SecretKey) == 0 {
		return fmt.Errorf("missing server.auth.secretKey")
	}

	return nil
}

// authenticateRPC authenticates RPCs using a middleware
func (s *Server) authenticateRPC(ctx context.Context, req *http.Request) (any, error) {
	// fetch value
	val := req.Header.Get(s.config.RPC.Auth.Header)
	if len(val) == 0 {
		return nil, authn.Errorf("missing %s header", s.config.RPC.Auth.Header)
	}

	// currently only static key comparison is supported:
	err := s.authenticateStaticSecretKey(val)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	return nil, nil
}

// authenticateStaticSecretKey authenticates a user provided value val
// using a static secret key.
func (s *Server) authenticateStaticSecretKey(val string) error {
	// strip scheme, trim space
	val, _ = strings.CutPrefix(val, s.config.RPC.Auth.Scheme+" ")
	val = strings.TrimSpace(val)

	if subtle.ConstantTimeCompare(
		[]byte(val),
		[]byte(s.config.RPC.Auth.SecretKey)) != 1 {
		return ErrInvalidAuthCredentials
	}

	return nil
}
