// Package config reads the service configuration from the environment.
package config

import (
	"fmt"
	"time"
)

// Config is the service configuration.
type Config struct {
	Port             string
	DatabaseURL      string
	PricingURL       string
	PricingTimeout   time.Duration
	InventoryURL     string
	InventoryTimeout time.Duration
}

// Load reads the configuration through getenv (os.Getenv in main).
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Port:         or(getenv("PORT"), "8080"),
		DatabaseURL:  getenv("DATABASE_URL"),
		PricingURL:   or(getenv("PRICING_URL"), "http://pricing.internal:8080"),
		InventoryURL: or(getenv("INVENTORY_URL"), "http://inventory.internal:8080"),
	}
	var err error
	if cfg.PricingTimeout, err = duration(getenv, "PRICING_TIMEOUT", 5*time.Second); err != nil {
		return cfg, err
	}
	if cfg.InventoryTimeout, err = duration(getenv, "INVENTORY_TIMEOUT", 3*time.Second); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func duration(getenv func(string) string, key string, def time.Duration) (time.Duration, error) {
	v := getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}
