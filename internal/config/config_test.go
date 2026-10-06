package config

import (
	"errors"
	"testing"
	"time"
)

func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load(lookupFrom(nil))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := Default()
	if cfg != want {
		t.Fatalf("Load() = %+v, want %+v", cfg, want)
	}
	if cfg.Addr != ":8080" {
		t.Fatalf("default Addr = %q", cfg.Addr)
	}
}

func TestLoad_Overrides(t *testing.T) {
	cfg, err := Load(lookupFrom(map[string]string{
		"ADDR":             ":9000",
		"READ_TIMEOUT":     "3s",
		"WRITE_TIMEOUT":    "4s",
		"IDLE_TIMEOUT":     "1m",
		"SHUTDOWN_TIMEOUT": "7s",
		"MAX_BODY_BYTES":   "2048",
		"LOG_LEVEL":        "debug",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Addr != ":9000" || cfg.ReadTimeout != 3*time.Second || cfg.WriteTimeout != 4*time.Second ||
		cfg.IdleTimeout != time.Minute || cfg.ShutdownTimeout != 7*time.Second ||
		cfg.MaxBodyBytes != 2048 || cfg.LogLevel != "debug" {
		t.Fatalf("Load() = %+v", cfg)
	}
}

func TestLoad_InvalidValues(t *testing.T) {
	tests := map[string]string{
		"READ_TIMEOUT":   "soon",
		"IDLE_TIMEOUT":   "-1s",
		"MAX_BODY_BYTES": "lots",
		"LOG_LEVEL":      "chatty",
	}
	for key, value := range tests {
		t.Run(key, func(t *testing.T) {
			_, err := Load(lookupFrom(map[string]string{key: value}))
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestConfig_SlogLevel(t *testing.T) {
	for in, want := range map[string]string{"debug": "DEBUG", "info": "INFO", "warn": "WARN", "error": "ERROR"} {
		cfg := Default()
		cfg.LogLevel = in
		if got := cfg.SlogLevel().String(); got != want {
			t.Errorf("SlogLevel(%q) = %s, want %s", in, got, want)
		}
	}
}
