package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const (
	defaultPort               = "8080"
	defaultServiceName        = "Redbridge Waste Collection"
	defaultTimezone           = "Europe/London"
	defaultCollectionSelector = ".collection"
	defaultDateSelector       = ".collection__date"
	defaultTypeSelector       = ".collection__type"
	defaultUserAgent          = "redbridge-council-rubbish-scraper/1.0"
)

var (
	defaultCacheTTL      = 12 * time.Hour
	defaultScrapeTimeout = 10 * time.Second
)

// Config centralises 12-factor friendly runtime configuration.
type Config struct {
	Port               string
	ScheduleURL        string
	CollectionSelector string
	DateSelector       string
	TypeSelector       string
	ServiceName        string
	Timezone           string
	UserAgent          string
	CacheTTL           time.Duration
	ScrapeTimeout      time.Duration
}

// Load builds the Config using environment variables.
func Load() (Config, error) {
	cacheTTL, err := readDuration("CACHE_TTL", defaultCacheTTL)
	if err != nil {
		return Config{}, err
	}

	timeout, err := readDuration("SCRAPE_TIMEOUT", defaultScrapeTimeout)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Port:               getEnv("PORT", defaultPort),
		ScheduleURL:        os.Getenv("SCRAPE_URL"),
		CollectionSelector: getEnv("COLLECTION_SELECTOR", defaultCollectionSelector),
		DateSelector:       getEnv("DATE_SELECTOR", defaultDateSelector),
		TypeSelector:       getEnv("TYPE_SELECTOR", defaultTypeSelector),
		ServiceName:        getEnv("SERVICE_NAME", defaultServiceName),
		Timezone:           getEnv("TIMEZONE", defaultTimezone),
		UserAgent:          getEnv("USER_AGENT", defaultUserAgent),
		CacheTTL:           cacheTTL,
		ScrapeTimeout:      timeout,
	}

	if cfg.ScheduleURL == "" {
		return Config{}, errors.New("SCRAPE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func readDuration(key string, fallback time.Duration) (time.Duration, error) {
	val := os.Getenv(key)
	if val == "" {
		return fallback, nil
	}

	d, err := time.ParseDuration(val)
	if err != nil {
		return 0, fmt.Errorf("invalid duration for %s: %w", key, err)
	}

	return d, nil
}
