package config

import (
	"time"

	"github.com/HMasataka/conic/internal/logging"
)

// Config represents the application configuration
type Config struct {
	Server  ServerConfig  `json:"server" yaml:"server"`
	WebRTC  WebRTCConfig  `json:"webrtc" yaml:"webrtc"`
	Logging logging.Config `json:"logging" yaml:"logging"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Host         string        `json:"host" yaml:"host"`
	Port         int           `json:"port" yaml:"port"`
	ReadTimeout  time.Duration `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`
	IdleTimeout  time.Duration `json:"idle_timeout" yaml:"idle_timeout"`
}

// WebRTCConfig represents WebRTC configuration
type WebRTCConfig struct {
	ICEServers []ICEServer `json:"ice_servers" yaml:"ice_servers"`
}

// ICEServer represents an ICE server configuration
type ICEServer struct {
	URLs       []string `json:"urls" yaml:"urls"`
	Username   string   `json:"username,omitempty" yaml:"username,omitempty"`
	Credential string   `json:"credential,omitempty" yaml:"credential,omitempty"`
}

// Default returns the default configuration
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         "localhost",
			Port:         3000,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		WebRTC: WebRTCConfig{
			ICEServers: []ICEServer{
				{
					URLs: []string{"stun:stun.l.google.com:19302"},
				},
			},
		},
		Logging: logging.Config{
			Level:  "info",
			Format: "json",
		},
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return NewConfigError("server.port", "invalid port number")
	}

	if c.Server.ReadTimeout < 0 {
		return NewConfigError("server.read_timeout", "timeout cannot be negative")
	}

	if c.Server.WriteTimeout < 0 {
		return NewConfigError("server.write_timeout", "timeout cannot be negative")
	}

	if len(c.WebRTC.ICEServers) == 0 {
		return NewConfigError("webrtc.ice_servers", "at least one ICE server is required")
	}

	return nil
}