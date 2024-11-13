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
	"time"

	"github.com/choopm/stdfx/loggingfx"
)

// Config holds the required config for [New]
type Config struct {
	// Log stores logging config
	Log loggingfx.Config `mapstructure:"log"`

	// KNX is the KNX bus config, required
	KNX KNXConfig `mapstructure:"knx"`

	// RPC is the rpc config, required
	RPC RPCConfig `mapstructure:"rpc"`

	// Client is the client config to test the server, optional
	Client ClientConfig `mapstructure:"knxrpc"`
}

// Validate validates the Config
func (c *Config) Validate() error {
	if err := c.KNX.Validate(); err != nil {
		return err
	}
	if err := c.RPC.Validate(); err != nil {
		return err
	}

	return nil
}

// KNXConfig holds the KNX bus config
type KNXConfig struct {
	// GatwewayHost is the Host or IP address of a KNX gateway, required
	GatwewayHost string `mapstructure:"gatewayHost"`

	// GatwewayPort is the port to use when communicating, defaults to 3671
	GatwewayPort int `mapstructure:"gatewayPort" default:"3671"`

	// Timeout is the default timeout for any bus activity or operation
	Timeout time.Duration `mapstructure:"timeout" default:"10s"`

	// InactivityTimeout is the timeout after which the servers errors if no bus activity was seen
	InactivityTimeout time.Duration `mapstructure:"inactivityTimeout" default:"5m"`

	// SendLocalAddress sends the local address when establishing a tunnel (breaks NAT)
	SendLocalAddress bool `mapstructure:"sendLocalAddress" default:"false"`

	// UseTCP establishes the tunnel using tcp instead of udp
	UseTCP bool `mapstructure:"useTCP" default:"false"`
}

// Validate validates the KNXConfig
func (c *KNXConfig) Validate() error {
	if len(c.GatwewayHost) == 0 {
		return fmt.Errorf("missing knx.gatewayHost")
	}
	if c.GatwewayPort == 0 {
		return fmt.Errorf("missing knx.gatewayPort")
	}

	return nil
}

// RPCConfig holds the RPC config
type RPCConfig struct {
	// Auth config to use
	Auth AuthConfig `mapstructure:"auth"`

	// Webserver config to use
	Webserver WebserverConfig `mapstructure:"webserver"`
}

// Validate validates the RPCConfig
func (c *RPCConfig) Validate() error {
	if err := c.Auth.Validate(); err != nil {
		return err
	}
	if err := c.Auth.Validate(); err != nil {
		return err
	}

	return nil
}

// WebserverConfig holds the webserver config
type WebserverConfig struct {
	// Enabled whether to start a http listener for the rpc server
	Enabled bool `mapstructure:"enabled" default:"false"`

	// Host is the listening host to use when starting a server
	Host string `mapstructure:"host" default:"127.0.0.1"`

	// Port is the listening port to use when starting a server
	Port int `mapstructure:"port" default:"8080"`

	// LogRequests whether to log requests
	LogRequests bool `mapstructure:"logRequests"`

	// Swagger config to use
	Swagger SwaggerConfig `mapstructure:"swagger"`

	// Metrics config to use
	Metrics MetricsConfig `mapstructure:"metrics"`
}

// Validate validates the HTTPConfig
func (c *WebserverConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	if len(c.Host) == 0 {
		return fmt.Errorf("missing webserver.host")
	}
	if c.Port == 0 {
		return fmt.Errorf("missing webserver.port")
	}
	if err := c.Swagger.Validate(); err != nil {
		return err
	}
	if err := c.Metrics.Validate(); err != nil {
		return err
	}

	return nil
}

// SwaggerConfig holds the swagger configuration
type SwaggerConfig struct {
	// Enabled whether to serve swaggerui
	Enabled bool `mapstructure:"enabled" default:"false"`

	// Path to serve swagger on
	Path string `mapstructure:"path" default:"/swagger"`

	// RootRedirect redirects / to <Path>/ if enabled
	RootRedirect bool `mapstructure:"rootRedirect" default:"false"`
}

// Validate validates the SwaggerConfig
func (c *SwaggerConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	if len(c.Path) == 0 {
		return fmt.Errorf("missing webserver.swagger.path")
	}

	return nil
}

// MetricsConfig holds the metrics configuration
type MetricsConfig struct {
	// Enabled whether to serve swaggerui
	Enabled bool `mapstructure:"enabled" default:"false"`

	// Path to serve mectris on
	Path string `mapstructure:"path" default:"/metrics"`

	// Auth config to use
	Auth AuthConfig `mapstructure:"auth"`
}

// Validate validates the MetricsConfig
func (c *MetricsConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	if len(c.Path) == 0 {
		return fmt.Errorf("missing webserver.metrics.path")
	}
	if err := c.Auth.Validate(); err != nil {
		return err
	}

	return nil
}

// ClientConfig holds the knxrpc client config
type ClientConfig struct {
	// Host is the knxrpc host to connect to
	Host string `mapstructure:"host"`

	// Port is the knxrpc host to connect to
	Port int `mapstructure:"port"`

	// Auth config to use
	Auth AuthConfig `mapstructure:"auth"`

	// UseTLS whether to connect using TLS
	UseTLS bool `mapstructure:"useTLS"`

	// InsecureTLS whether to use insecureSkipVerify
	InsecureTLS bool `mapstructure:"insecureTLS"`
}

// Validate validates the ClientConfig
func (c *ClientConfig) Validate() error {
	if len(c.Host) == 0 {
		return fmt.Errorf("missing knxrpc.host")
	}
	if c.Port == 0 {
		return fmt.Errorf("missing knxrpc.port")
	}
	if err := c.Auth.Validate(); err != nil {
		return err
	}

	return nil
}
