package config_test

import (
	"testing"

	"github.com/fabiant7t/hashrouter/internal/config"
)

func TestNew(t *testing.T) {
	t.Parallel()

	cfg := config.New()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	if cfg.Debug {
		t.Fatalf("debug mismatch: got %t want %t", cfg.Debug, false)
	}
}

func TestNewFromEnv(t *testing.T) {
	t.Run("DEBUG unset", func(t *testing.T) {
		t.Setenv("DEBUG", "")

		cfg := config.NewFromEnv()
		if cfg.Debug {
			t.Fatalf("debug mismatch: got %t want %t", cfg.Debug, false)
		}
	})

	t.Run("DEBUG true", func(t *testing.T) {
		t.Setenv("DEBUG", "true")

		cfg := config.NewFromEnv()
		if !cfg.Debug {
			t.Fatalf("debug mismatch: got %t want %t", cfg.Debug, true)
		}
	})

	t.Run("DEBUG false", func(t *testing.T) {
		t.Setenv("DEBUG", "false")

		cfg := config.NewFromEnv()
		if cfg.Debug {
			t.Fatalf("debug mismatch: got %t want %t", cfg.Debug, false)
		}
	})

	t.Run("DEBUG invalid", func(t *testing.T) {
		t.Setenv("DEBUG", "not-a-bool")

		cfg := config.NewFromEnv()
		if cfg.Debug {
			t.Fatalf("debug mismatch: got %t want %t", cfg.Debug, false)
		}
	})
}
