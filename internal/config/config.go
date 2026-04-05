// Package config provides configuration management for the application.
//
//nolint:tagliatelle // project convention
package config

import (
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.yaml.in/yaml/v2"
)

const (
	defaultPath            = "config.yaml"
	defaultShutdownTimeout = 5 * time.Second
)

// Settings holds the application settings.
type Settings struct {
	// Host is the server hostname of the application, different from HTTP.Host.
	Host string `json:"host" yaml:"host"`
	// HTTP holds the configuration for the HTTP server.
	HTTP HTTPSettings `json:"http" yaml:"http"`
	// Log holds the configuration for the logger.
	Log Log `json:"log"  yaml:"log"`

	// path is the path to the configuration file.
	path string
	// ctx is the context for the settings, used for cancellation and deadlines.
	ctx context.Context
}

// HTTPSettings holds the configuration for the HTTP server.
type HTTPSettings struct {
	// Host is the hostname for the HTTP server.
	Host string `json:"host" yaml:"host"`
	// Port is the port for the HTTP server.
	Port int `json:"port" yaml:"port"`
	// ShutdownTimeout is the duration to wait for the HTTP server to shut down gracefully.
	ShutdownTimeout time.Duration `json:"shutdown_timeout" yaml:"shutdown_timeout"`
}

// Address returns the HTTP listen address in "host:port" format.
func (h *HTTPSettings) Address() string {
	return net.JoinHostPort(h.Host, fmt.Sprintf("%d", h.Port))
}

// Validate checks the Settings for required fields and returns an error if any are missing.
func (s *Settings) Validate() error {
	if s.Host == "" {
		return fmt.Errorf("host is required")
	}

	if s.HTTP.Host == "" {
		return fmt.Errorf("http.host is required")
	}

	if s.HTTP.Port == 0 {
		return fmt.Errorf("http.port is required")
	}

	if s.HTTP.ShutdownTimeout == 0 {
		s.HTTP.ShutdownTimeout = defaultShutdownTimeout
	}

	return nil

}

// Options defines a function type for setting options.
type Options func(*Settings) error

// WithPath sets the configuration file path.
func WithPath(path string) Options {
	return func(s *Settings) error {
		s.path = path
		return nil
	}
}

// WithContext sets a context for the level watcher.
func WithContext(ctx context.Context) Options {
	return func(s *Settings) error {
		s.ctx = ctx
		return nil
	}
}

// New creates a new Settings instance with the provided options.
func New(opts ...Options) (*Settings, error) {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	settings := &Settings{
		Host: hostname,
		Log: Log{
			Level: zerolog.InfoLevel,
		},
		path: defaultPath,
	}

	for _, opt := range opts {
		if err := opt(settings); err != nil {
			return nil, err
		}
	}

	if err := settings.Load(); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %v", err)
	}

	log.Debug().
		Any("config", settings).
		Msg("loaded configuration file")

	settings.setLogger()

	return settings, nil
}

// setLogger sets the logger given the configuration
func (s *Settings) setLogger() {
	log.Logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	zerolog.SetGlobalLevel(s.Log.Level)
}

// Log holds the configuration for the logger.
type Log struct {
	Level zerolog.Level `json:"level" yaml:"level"`
}

// Load loads the configuration from a file
func (s *Settings) Load() error {
	if err := s.readFile(); err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	return nil
}

// readFile reads the configuration file and returns the Config object
func (s *Settings) readFile() error {
	contents, err := os.ReadFile(path.Clean(s.path))
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	if err := s.unmarshal(contents); err != nil {
		return err
	}
	return nil

}

// unmarshal unmarshals the byte slice into the Settings object
func (s *Settings) unmarshal(bytes []byte) error {
	if err := yaml.Unmarshal(bytes, s); err != nil {
		return fmt.Errorf("error unmarshaling config: %w", err)
	}

	if err := s.Validate(); err != nil {
		return fmt.Errorf("error validating config: %w", err)
	}

	zerolog.SetGlobalLevel(s.Log.Level)

	return nil
}
