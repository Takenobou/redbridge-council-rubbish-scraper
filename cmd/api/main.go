package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Takenobou/redbridge-council-rubbish-scraper/internal/calendar"
	"github.com/Takenobou/redbridge-council-rubbish-scraper/internal/config"
	"github.com/Takenobou/redbridge-council-rubbish-scraper/internal/scraper"
	"github.com/Takenobou/redbridge-council-rubbish-scraper/internal/server"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	scraperClient, err := scraper.New(scraper.Config{
		BaseURL:        cfg.BaseURL,
		SchedulePath:   cfg.SchedulePath,
		UPRN:           cfg.UPRN,
		AddressLine:    cfg.AddressLine,
		Postcode:       cfg.Postcode,
		Latitude:       cfg.Latitude,
		Longitude:      cfg.Longitude,
		UserAgent:      cfg.UserAgent,
		StartHour:      cfg.StartHour,
		RequestTimeout: cfg.RequestTimeout,
		Timezone:       cfg.Timezone,
	})
	if err != nil {
		logger.Error("scraper init failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	calendarBuilder, err := calendar.NewBuilder(calendar.Config{
		Name:        cfg.CalendarName,
		Description: cfg.CalendarDesc,
		Timezone:    cfg.Timezone,
	})
	if err != nil {
		logger.Error("calendar init failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	srv := server.New(cfg, scraperClient, calendarBuilder, logger)

	if err := srv.Run(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server exited with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
