package config

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const DefaultAPIBaseURL = "https://api.openweathermap.org/data/2.5/weather"

type Config struct {
	APIKey        string
	Latitude      float64
	Longitude     float64
	DatabaseURL   string
	APIBaseURL    string
	Interval      time.Duration
	HTTPTimeout   time.Duration
	HTTPAddr      string
	DataDir       string
	LogFile       string
	LogStdout     bool
	ShutdownGrace time.Duration
}

type LookupEnv func(string) (string, bool)

func FromEnvironment(lookup LookupEnv) (Config, error) {
	cfg := Config{
		APIBaseURL: DefaultAPIBaseURL, Interval: 30 * time.Second,
		HTTPTimeout: 10 * time.Second, HTTPAddr: ":8080", DataDir: "data",
		LogFile: "logs/etl.log", ShutdownGrace: 10 * time.Second,
	}
	if lookup == nil {
		return cfg, errors.New("environment lookup is required")
	}
	cfg.APIKey = value(lookup, "OPENWEATHER_API_KEY", "")
	cfg.DatabaseURL = value(lookup, "DATABASE_URL", "")
	cfg.APIBaseURL = value(lookup, "OPENWEATHER_BASE_URL", cfg.APIBaseURL)
	cfg.HTTPAddr = value(lookup, "HTTP_ADDR", cfg.HTTPAddr)
	cfg.DataDir = value(lookup, "DATA_DIR", cfg.DataDir)
	cfg.LogFile = value(lookup, "LOG_FILE", cfg.LogFile)
	cfg.LogStdout = strings.EqualFold(value(lookup, "LOG_STDOUT", "false"), "true")

	var err error
	if cfg.Latitude, err = requiredFloat(lookup, "WEATHER_LAT"); err != nil {
		return Config{}, err
	}
	if cfg.Longitude, err = requiredFloat(lookup, "WEATHER_LON"); err != nil {
		return Config{}, err
	}
	if cfg.Interval, err = duration(lookup, "ETL_INTERVAL", cfg.Interval); err != nil {
		return Config{}, err
	}
	if cfg.HTTPTimeout, err = duration(lookup, "HTTP_TIMEOUT", cfg.HTTPTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownGrace, err = duration(lookup, "SHUTDOWN_GRACE", cfg.ShutdownGrace); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func value(lookup LookupEnv, key, fallback string) string {
	if v, ok := lookup(key); ok {
		return strings.TrimSpace(v)
	}
	return fallback
}

func requiredFloat(lookup LookupEnv, key string) (float64, error) {
	v := value(lookup, key, "")
	if v == "" {
		return 0, fmt.Errorf("%s is required", key)
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number", key)
	}
	return n, nil
}

func duration(lookup LookupEnv, key string, fallback time.Duration) (time.Duration, error) {
	v := value(lookup, key, "")
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return d, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.APIKey) == "" {
		return errors.New("OPENWEATHER_API_KEY is required")
	}
	if math.IsNaN(c.Latitude) || math.IsInf(c.Latitude, 0) || c.Latitude < -90 || c.Latitude > 90 {
		return errors.New("WEATHER_LAT must be between -90 and 90")
	}
	if math.IsNaN(c.Longitude) || math.IsInf(c.Longitude, 0) || c.Longitude < -180 || c.Longitude > 180 {
		return errors.New("WEATHER_LON must be between -180 and 180")
	}
	if c.Interval <= 0 {
		return errors.New("ETL_INTERVAL must be positive")
	}
	if c.HTTPTimeout <= 0 {
		return errors.New("HTTP_TIMEOUT must be positive")
	}
	if c.ShutdownGrace <= 0 {
		return errors.New("SHUTDOWN_GRACE must be positive")
	}
	if err := validateURL(c.APIBaseURL, true); err != nil {
		return errors.New("OPENWEATHER_BASE_URL must be a valid HTTP(S) URL")
	}
	if err := validateURL(c.DatabaseURL, false); err != nil {
		return errors.New("DATABASE_URL must be a valid PostgreSQL URL")
	}
	if _, err := net.ResolveTCPAddr("tcp", c.HTTPAddr); err != nil {
		return errors.New("HTTP_ADDR must be a valid TCP listen address")
	}
	if err := validPath(c.DataDir); err != nil {
		return errors.New("DATA_DIR must be a usable path")
	}
	if err := validPath(c.LogFile); err != nil {
		return errors.New("LOG_FILE must be a usable path")
	}
	return nil
}

func validateURL(raw string, httpOnly bool) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New("empty")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return errors.New("invalid")
	}
	if httpOnly && u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("scheme")
	}
	if !httpOnly && u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return errors.New("scheme")
	}
	return nil
}

func validPath(path string) error {
	if strings.TrimSpace(path) == "" || strings.ContainsRune(path, '\x00') {
		return errors.New("invalid")
	}
	_, err := filepath.Abs(path)
	return err
}
