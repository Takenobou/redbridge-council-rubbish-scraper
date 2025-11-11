package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/Takenobou/redbridge-council-rubbish-scraper/internal/config"
	"github.com/Takenobou/redbridge-council-rubbish-scraper/internal/scraper"
)

// Scraper abstracts collection lookups for easier testing.
type Scraper interface {
	FetchCollections(context.Context) ([]scraper.Collection, error)
}

// CalendarBuilder abstracts ICS generation.
type CalendarBuilder interface {
	Build([]scraper.Collection) ([]byte, error)
}

// Server wires together HTTP endpoints, the scraper, and the calendar builder.
type Server struct {
	cfg        config.Config
	scraper    Scraper
	calendar   CalendarBuilder
	logger     *slog.Logger
	httpServer *http.Server
	cache      *calendarCache
}

// New prepares a Server for use.
func New(cfg config.Config, scr Scraper, cal CalendarBuilder, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	s := &Server{
		cfg:      cfg,
		scraper:  scr,
		calendar: cal,
		logger:   logger,
		cache:    newCalendarCache(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthHandler)
	mux.HandleFunc("GET /calendar.ics", s.calendarHandler)

	s.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return s
}

// Run starts the HTTP server and blocks until shutdown.
func (s *Server) Run(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			s.logger.Error("graceful shutdown failed", slog.String("error", err.Error()))
		}
	}()

	return s.httpServer.ListenAndServe()
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

func (s *Server) calendarHandler(w http.ResponseWriter, r *http.Request) {
	if data, ok := s.cache.Get(); ok {
		s.respondICS(w, data)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.ScrapeTimeout)
	defer cancel()

	collections, err := s.scraper.FetchCollections(ctx)
	if err != nil {
		s.logger.Error("scrape failed", slog.String("error", err.Error()))
		writeProblem(w, http.StatusBadGateway, "scrape_failed", "unable to fetch latest collections")
		return
	}

	payload, err := s.calendar.Build(collections)
	if err != nil {
		s.logger.Error("calendar build failed", slog.String("error", err.Error()))
		writeProblem(w, http.StatusInternalServerError, "calendar_failed", "unable to generate calendar")
		return
	}

	s.cache.Set(payload, s.cfg.CacheTTL)
	s.respondICS(w, payload)
}

func (s *Server) respondICS(w http.ResponseWriter, payload []byte) {
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(s.cfg.CacheTTL.Seconds())))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(payload); err != nil {
		s.logger.Warn("failed to write response", slog.String("error", err.Error()))
	}
}

func writeProblem(w http.ResponseWriter, status int, code, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"code":   code,
		"detail": detail,
	})
}

type calendarCache struct {
	mu        sync.RWMutex
	payload   []byte
	expiresAt time.Time
}

func newCalendarCache() *calendarCache {
	return &calendarCache{
		payload: nil,
	}
}

func (c *calendarCache) Get() ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.payload == nil || time.Now().After(c.expiresAt) {
		return nil, false
	}
	return append([]byte(nil), c.payload...), true
}

func (c *calendarCache) Set(payload []byte, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.payload = append([]byte(nil), payload...)
	c.expiresAt = time.Now().Add(ttl)
}
