package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL       = "https://my.redbridge.gov.uk"
	defaultSchedulePath  = "/RecycleRefuse"
	defaultUserAgent     = "redbridge-council-rubbish-scraper/1.0"
	defaultCacheTTL      = 168 * time.Hour
	defaultRequestTimout = 15 * time.Second
	defaultStartHour     = 6
	defaultListenAddr    = ":8080"
	londonTimezone       = "Europe/London"
	calendarName         = "Redbridge Collections"
	calendarDescription  = "Household waste & recycling (scraped)"
)

// Config centralises 12-factor friendly runtime configuration.
type Config struct {
	ListenAddr     string
	BaseURL        string
	SchedulePath   string
	UPRN           string
	AddressLine    string
	Postcode       string
	Latitude       string
	Longitude      string
	CacheTTL       time.Duration
	StartHour      int
	UserAgent      string
	RequestTimeout time.Duration
	Timezone       string
	CalendarName   string
	CalendarDesc   string
}

// Load builds the Config using environment variables.
func Load() (Config, error) {
	cacheTTL, err := readDuration("CACHE_TTL", defaultCacheTTL)
	if err != nil {
		return Config{}, err
	}

	timeout, err := readDuration("SCRAPE_TIMEOUT", defaultRequestTimout)
	if err != nil {
		return Config{}, err
	}

	startHour, err := readInt("START_HOUR", defaultStartHour)
	if err != nil {
		return Config{}, err
	}
	if startHour < 0 || startHour > 23 {
		return Config{}, fmt.Errorf("START_HOUR must be between 0 and 23")
	}

	cfg := Config{
		ListenAddr:     getEnv("LISTEN_ADDR", defaultListenAddr),
		BaseURL:        strings.TrimRight(getEnv("BASE_URL", defaultBaseURL), "/"),
		SchedulePath:   ensurePath(getEnv("SCHEDULE_PATH", defaultSchedulePath)),
		UPRN:           os.Getenv("UPRN"),
		AddressLine:    os.Getenv("ADDRESS_LINE"),
		Postcode:       os.Getenv("POSTCODE"),
		Latitude:       os.Getenv("LATITUDE"),
		Longitude:      os.Getenv("LONGITUDE"),
		CacheTTL:       cacheTTL,
		StartHour:      startHour,
		UserAgent:      getEnv("USER_AGENT", defaultUserAgent),
		RequestTimeout: timeout,
		Timezone:       londonTimezone,
		CalendarName:   calendarName,
		CalendarDesc:   calendarDescription,
	}

	if cfg.UPRN == "" {
		return Config{}, errors.New("UPRN is required")
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

func readInt(key string, fallback int) (int, error) {
	val := os.Getenv(key)
	if val == "" {
		return fallback, nil
	}

	i, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid integer for %s: %w", key, err)
	}

	return i, nil
}

func ensurePath(p string) string {
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, "/") {
		return p
	}
	return "/" + p
}
