package config

import (
	"fmt"
	"strconv"
	"strings"
)

// Config holds all application configuration.
type Config struct {
	DBPath                 string
	GoogleAPIKey           string
	HomeLatitude           float64
	HomeLongitude          float64
	HomeAddress            string
	WebPort                int
	DryRun                 bool
	ErrorDiscordWebhookURL string
	EnabledHunts           map[string]bool
	HuntWebhooks           map[string]string
}

// Known hunt names for config parsing.
// knownHunts maps config key → hunt name. Config keys are used for env vars
// (e.g., HUNT_PERFORMING_ENABLED), hunt names are used for EnabledHunts map keys.
var knownHunts = map[string]string{
	"comedy":     "comedy",
	"performing": "performing-arts",
	"powder":     "powder",
	"movies":     "movies",
}

// FromEnv parses configuration from a lookup function (typically os.Getenv).
func FromEnv(lookup func(string) string) (Config, error) {
	cfg := Config{
		DBPath:       orDefault(lookup("DB_PATH"), "opportunity-hunter.db"),
		GoogleAPIKey: lookup("GOOGLE_API_KEY"),
		HomeAddress:  lookup("HOME_ADDRESS"),
		WebPort:      parseIntOr(lookup("WEB_PORT"), 8080),
		DryRun:       parseBool(lookup("DRY_RUN")),
		ErrorDiscordWebhookURL: lookup("ERROR_DISCORD_WEBHOOK_URL"),
		EnabledHunts: make(map[string]bool),
		HuntWebhooks: make(map[string]string),
	}

	if cfg.GoogleAPIKey == "" {
		return cfg, fmt.Errorf("GOOGLE_API_KEY is required")
	}

	cfg.HomeLatitude = parseFloatOr(lookup("HOME_LATITUDE"), 39.75)
	cfg.HomeLongitude = parseFloatOr(lookup("HOME_LONGITUDE"), -104.99)

	// Parse per-hunt toggles.
	for configKey, huntName := range knownHunts {
		key := "HUNT_" + strings.ToUpper(configKey) + "_ENABLED"
		val := lookup(key)
		if val == "" {
			cfg.EnabledHunts[huntName] = true // enabled by default
		} else {
			cfg.EnabledHunts[huntName] = parseBool(val)
		}
	}

	// Parse per-hunt webhooks.
	for configKey, huntName := range knownHunts {
		key := strings.ToUpper(configKey) + "_DISCORD_WEBHOOK_URL"
		if url := lookup(key); url != "" {
			cfg.HuntWebhooks[huntName] = url
		}
	}

	return cfg, nil
}

func orDefault(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

func parseIntOr(val string, def int) int {
	if val == "" {
		return def
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return def
	}
	return n
}

func parseFloatOr(val string, def float64) float64 {
	if val == "" {
		return def
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return def
	}
	return f
}

func parseBool(val string) bool {
	return strings.EqualFold(val, "true") || val == "1"
}
