package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadOptions represents options for loading configuration
type LoadOptions struct {
	Path        string
	Environment string
}

// Load loads configuration from various sources
func Load(opts ...LoadOptions) (*Config, error) {
	cfg := Default()

	// Apply options
	var options LoadOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	// Load from file if path is specified
	if options.Path != "" {
		if err := loadFromFile(cfg, options.Path); err != nil {
			return nil, err
		}
	}

	// Override with environment variables
	loadFromEnv(cfg)

	// Validate the final configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// loadFromFile loads configuration from a file
func loadFromFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, cfg); err != nil {
			return fmt.Errorf("failed to parse JSON config: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return fmt.Errorf("failed to parse YAML config: %w", err)
		}
	default:
		return fmt.Errorf("unsupported config file format: %s", ext)
	}

	return nil
}

// loadFromEnv loads configuration from environment variables
func loadFromEnv(cfg *Config) {
	// Server configuration
	if host := os.Getenv("CONIC_SERVER_HOST"); host != "" {
		cfg.Server.Host = host
	}
	if port := os.Getenv("CONIC_SERVER_PORT"); port != "" {
		if p, err := parseInt(port); err == nil {
			cfg.Server.Port = p
		}
	}

	// Logging configuration
	if level := os.Getenv("CONIC_LOG_LEVEL"); level != "" {
		cfg.Logging.Level = level
	}
	if format := os.Getenv("CONIC_LOG_FORMAT"); format != "" {
		cfg.Logging.Format = format
	}

	// WebRTC configuration
	if iceServers := os.Getenv("CONIC_ICE_SERVERS"); iceServers != "" {
		// Parse comma-separated ICE server URLs
		urls := strings.Split(iceServers, ",")
		if len(urls) > 0 {
			cfg.WebRTC.ICEServers = []ICEServer{
				{URLs: urls},
			}
		}
	}
}

// parseInt parses a string to int
func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

// ConfigError represents a configuration error
type ConfigError struct {
	Field   string
	Message string
}

// NewConfigError creates a new configuration error
func NewConfigError(field, message string) *ConfigError {
	return &ConfigError{
		Field:   field,
		Message: message,
	}
}

// Error implements the error interface
func (e *ConfigError) Error() string {
	return fmt.Sprintf("config error in field '%s': %s", e.Field, e.Message)
}
