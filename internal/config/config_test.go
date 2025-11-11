package config

import "testing"

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("UPRN", "123")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.ListenAddr != ":8080" {
		t.Fatalf("expected default listen addr, got %s", cfg.ListenAddr)
	}
	if cfg.CacheTTL.Hours() != 168 {
		t.Fatalf("expected cache ttl 168h, got %s", cfg.CacheTTL)
	}
	if cfg.CalendarName == "" || cfg.CalendarDesc == "" {
		t.Fatalf("calendar metadata missing")
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	t.Setenv("UPRN", "123")
	t.Setenv("LISTEN_ADDR", "127.0.0.1:9090")
	t.Setenv("BASE_URL", "https://example.com/")
	t.Setenv("SCHEDULE_PATH", "custom")
	t.Setenv("CACHE_TTL", "24h")
	t.Setenv("START_HOUR", "7")
	t.Setenv("SCRAPE_TIMEOUT", "5s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.ListenAddr != "127.0.0.1:9090" {
		t.Fatalf("ListenAddr not overridden")
	}
	if cfg.BaseURL != "https://example.com" {
		t.Fatalf("BaseURL trimming failed: %s", cfg.BaseURL)
	}
	if cfg.SchedulePath != "/custom" {
		t.Fatalf("Schedule path not normalized: %s", cfg.SchedulePath)
	}
	if cfg.CacheTTL.Hours() != 24 {
		t.Fatalf("CacheTTL override failed: %s", cfg.CacheTTL)
	}
	if cfg.StartHour != 7 {
		t.Fatalf("StartHour override failed: %d", cfg.StartHour)
	}
	if cfg.RequestTimeout.String() != "5s" {
		t.Fatalf("RequestTimeout override failed: %s", cfg.RequestTimeout)
	}
}

func TestLoadConfigRequiresUPRN(t *testing.T) {
	t.Setenv("UPRN", "")
	if _, err := Load(); err == nil {
		t.Fatalf("expected error when UPRN missing")
	}
}
