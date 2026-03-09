package config_test

import (
	"testing"

	"github.com/seanmeyer/opportunity-hunter/config"
)

func fakeLookup(vals map[string]string) func(string) string {
	return func(key string) string { return vals[key] }
}

func TestFromEnv_Defaults(t *testing.T) {
	cfg, err := config.FromEnv(fakeLookup(map[string]string{
		"GOOGLE_API_KEY": "test-key",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WebPort != 8080 {
		t.Fatalf("expected default port 8080, got %d", cfg.WebPort)
	}
	if cfg.DBPath != "opportunity-hunter.db" {
		t.Fatalf("expected default DB path, got %q", cfg.DBPath)
	}
	if cfg.DryRun {
		t.Fatal("expected DryRun false by default")
	}
}

func TestFromEnv_MissingGoogleAPIKey(t *testing.T) {
	_, err := config.FromEnv(fakeLookup(map[string]string{}))
	if err == nil {
		t.Fatal("expected error for missing GOOGLE_API_KEY")
	}
}

func TestFromEnv_PerHuntEnabled(t *testing.T) {
	cfg, _ := config.FromEnv(fakeLookup(map[string]string{
		"GOOGLE_API_KEY":       "key",
		"HUNT_COMEDY_ENABLED":  "true",
		"HUNT_POWDER_ENABLED":  "false",
		"HUNT_MOVIES_ENABLED":  "true",
	}))

	if !cfg.EnabledHunts["comedy"] {
		t.Fatal("comedy should be enabled")
	}
	if cfg.EnabledHunts["powder"] {
		t.Fatal("powder should be disabled")
	}
	if !cfg.EnabledHunts["movies"] {
		t.Fatal("movies should be enabled")
	}
}

func TestFromEnv_PerHuntWebhooks(t *testing.T) {
	cfg, _ := config.FromEnv(fakeLookup(map[string]string{
		"GOOGLE_API_KEY":             "key",
		"COMEDY_DISCORD_WEBHOOK_URL": "https://discord.com/comedy",
		"POWDER_DISCORD_WEBHOOK_URL": "https://discord.com/powder",
	}))

	if cfg.HuntWebhooks["comedy"] != "https://discord.com/comedy" {
		t.Fatalf("expected comedy webhook, got %q", cfg.HuntWebhooks["comedy"])
	}
}

func TestFromEnv_CustomValues(t *testing.T) {
	cfg, _ := config.FromEnv(fakeLookup(map[string]string{
		"GOOGLE_API_KEY":  "key",
		"DB_PATH":         "/data/custom.db",
		"WEB_PORT":        "9090",
		"HOME_LATITUDE":   "40.0",
		"HOME_LONGITUDE":  "-105.0",
		"HOME_ADDRESS":    "456 Oak St",
		"DRY_RUN":         "true",
	}))

	if cfg.DBPath != "/data/custom.db" {
		t.Fatalf("expected custom DB path, got %q", cfg.DBPath)
	}
	if cfg.WebPort != 9090 {
		t.Fatalf("expected port 9090, got %d", cfg.WebPort)
	}
	if cfg.HomeLatitude != 40.0 {
		t.Fatalf("expected lat 40.0, got %f", cfg.HomeLatitude)
	}
	if !cfg.DryRun {
		t.Fatal("expected DryRun true")
	}
}
