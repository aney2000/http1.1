// Package config loads runtime settings from environment variables,
// following the twelve-factor app convention used by container platforms.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"
)

// ErrInvalid is returned when an environment variable cannot be parsed.
var ErrInvalid = errors.New("invalid configuration")

// Config holds all runtime settings.
type Config struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	MaxBodyBytes    int64
	LogLevel        string
}

// Default returns the configuration used when no variables are set.
func Default() Config {
	return Config{
		Addr:            ":8080",
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     60 * time.Second,
		ShutdownTimeout: 15 * time.Second,
		MaxBodyBytes:    10 * 1024 * 1024,
		LogLevel:        "info",
	}
}

var logLevels = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

// Load reads overrides through lookup (normally os.LookupEnv; injectable
// so tests never touch the real process environment).
func Load(lookup func(string) (string, bool)) (Config, error) {
	cfg := Default()
	l := loader{lookup: lookup}

	l.string("ADDR", &cfg.Addr)
	l.duration("READ_TIMEOUT", &cfg.ReadTimeout)
	l.duration("WRITE_TIMEOUT", &cfg.WriteTimeout)
	l.duration("IDLE_TIMEOUT", &cfg.IdleTimeout)
	l.duration("SHUTDOWN_TIMEOUT", &cfg.ShutdownTimeout)
	l.int64("MAX_BODY_BYTES", &cfg.MaxBodyBytes)
	l.string("LOG_LEVEL", &cfg.LogLevel)

	if _, ok := logLevels[cfg.LogLevel]; !ok && l.err == nil {
		l.err = fmt.Errorf("%w: LOG_LEVEL=%q", ErrInvalid, cfg.LogLevel)
	}
	return cfg, l.err
}

// SlogLevel converts LogLevel to a slog.Level.
func (c Config) SlogLevel() slog.Level {
	return logLevels[c.LogLevel]
}

// loader keeps the first error so Load reads as a flat list of settings.
type loader struct {
	lookup func(string) (string, bool)
	err    error
}

func (l *loader) string(key string, dst *string) {
	if v, ok := l.lookup(key); ok {
		*dst = v
	}
}

func (l *loader) duration(key string, dst *time.Duration) {
	v, ok := l.lookup(key)
	if !ok || l.err != nil {
		return
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		l.err = fmt.Errorf("%w: %s=%q", ErrInvalid, key, v)
		return
	}
	*dst = d
}

func (l *loader) int64(key string, dst *int64) {
	v, ok := l.lookup(key)
	if !ok || l.err != nil {
		return
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		l.err = fmt.Errorf("%w: %s=%q", ErrInvalid, key, v)
		return
	}
	*dst = n
}
