package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config contains process settings shared by the API and migration command.
type Config struct {
	DatabaseURL     string
	HTTPAddr        string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	MigrationsDir   string
}

func Load() (Config, error) { return LoadFrom(os.LookupEnv) }

func LoadFrom(lookup func(string) (string, bool)) (Config, error) {
	c := Config{
		HTTPAddr:        ":8080",
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    30 * time.Second,
		IdleTimeout:     60 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}
	databaseURL, hasURL := lookup("DATABASE_URL")
	databaseURLFile, hasURLFile := lookup("DATABASE_URL_FILE")
	if hasURL && hasURLFile {
		return Config{}, errors.New("set only one of DATABASE_URL or DATABASE_URL_FILE")
	}
	if hasURL {
		c.DatabaseURL = databaseURL
	} else if hasURLFile {
		contents, err := os.ReadFile(databaseURLFile)
		if err != nil {
			return Config{}, fmt.Errorf("read DATABASE_URL_FILE: %w", err)
		}
		c.DatabaseURL = strings.TrimSpace(string(contents))
	}
	if value, ok := lookup("LISTEN_ADDR"); ok && value != "" {
		c.HTTPAddr = value
	} else if value, ok := lookup("HTTP_ADDR"); ok && value != "" {
		c.HTTPAddr = value
	}
	if value, ok := lookup("MIGRATIONS_DIR"); ok {
		c.MigrationsDir = value
	}
	var err error
	if c.ReadTimeout, err = duration(lookup, "HTTP_READ_TIMEOUT", c.ReadTimeout); err != nil {
		return Config{}, err
	}
	if c.WriteTimeout, err = duration(lookup, "HTTP_WRITE_TIMEOUT", c.WriteTimeout); err != nil {
		return Config{}, err
	}
	if c.IdleTimeout, err = duration(lookup, "HTTP_IDLE_TIMEOUT", c.IdleTimeout); err != nil {
		return Config{}, err
	}
	if c.ShutdownTimeout, err = duration(lookup, "SHUTDOWN_TIMEOUT", c.ShutdownTimeout); err != nil {
		return Config{}, err
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func duration(lookup func(string) (string, bool), key string, fallback time.Duration) (time.Duration, error) {
	value, ok := lookup(key)
	if !ok || value == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", key, err)
	}
	return d, nil
}

func (c Config) Validate() error {
	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	u, err := url.Parse(c.DatabaseURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		return errors.New("DATABASE_URL must be a valid postgres:// or postgresql:// URL")
	}
	if c.HTTPAddr == "" {
		return errors.New("HTTP_ADDR must not be empty")
	}
	for key, d := range map[string]time.Duration{
		"HTTP_READ_TIMEOUT": c.ReadTimeout, "HTTP_WRITE_TIMEOUT": c.WriteTimeout,
		"HTTP_IDLE_TIMEOUT": c.IdleTimeout, "SHUTDOWN_TIMEOUT": c.ShutdownTimeout,
	} {
		if d <= 0 {
			return fmt.Errorf("%s must be positive", key)
		}
	}
	return nil
}
