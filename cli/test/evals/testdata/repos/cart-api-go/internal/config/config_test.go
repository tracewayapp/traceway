package config

import (
	"testing"
	"time"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoadOverrides(t *testing.T) {
	cfg, err := Load(env(map[string]string{"PRICING_URL": "http://localhost:9000", "PRICING_TIMEOUT": "750ms"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PricingURL != "http://localhost:9000" || cfg.PricingTimeout != 750*time.Millisecond {
		t.Fatalf("overrides not applied: %+v", cfg)
	}
}

func TestLoadRejectsBadDuration(t *testing.T) {
	if _, err := Load(env(map[string]string{"INVENTORY_TIMEOUT": "2"})); err == nil {
		t.Fatal("a duration without a unit must be rejected")
	}
}
