package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Takenobou/redbridge-council-rubbish-scraper/internal/config"
	"github.com/Takenobou/redbridge-council-rubbish-scraper/internal/scraper"
)

const (
	collectionDuration = time.Hour
	cacheControlICS    = "public, max-age=300"
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
	cache      *collectionCache
	location   *time.Location
}

// New prepares a Server for use.
func New(cfg config.Config, scr Scraper, cal CalendarBuilder, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	loc, _ := time.LoadLocation(cfg.Timezone)

	s := &Server{
		cfg:      cfg,
		scraper:  scr,
		calendar: cal,
		logger:   logger,
		cache:    newCollectionCache(),
		location: loc,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthHandler)
	mux.HandleFunc("GET /calendar.ics", s.calendarHandler)
	mux.HandleFunc("GET /api/next", s.nextHandler)
	mux.HandleFunc("GET /api/types", s.typesHandler)
	mux.HandleFunc("GET /api/is-today", s.isTodayHandler)
	mux.HandleFunc("GET /api/is-tomorrow", s.isTomorrowHandler)

	s.httpServer = &http.Server{
		Addr:              cfg.ListenAddr,
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

	s.logger.Info("listening", slog.String("addr", s.cfg.ListenAddr))
	return s.httpServer.ListenAndServe()
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) calendarHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	collections, err := s.collections(ctx)
	if err != nil {
		s.respondScrapeError(w, err)
		return
	}

	payload, err := s.calendar.Build(collections)
	if err != nil {
		s.logger.Error("calendar build failed", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "calendar_failed",
		})
		return
	}

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Cache-Control", cacheControlICS)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(payload); err != nil {
		s.logger.Warn("failed to write response", slog.String("error", err.Error()))
	}
}

func (s *Server) nextHandler(w http.ResponseWriter, r *http.Request) {
	now, ok := s.resolveNow(w, r.URL.Query())
	if !ok {
		return
	}

	collections, err := s.collections(r.Context())
	if err != nil {
		s.respondUnavailable(w, err)
		return
	}

	day, found := nextDay(now, collections, s.location)
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no_upcoming_collections"})
		return
	}

	days := daysBetween(now, day.Date, s.location)
	resp := map[string]interface{}{
		"date":  day.Date.In(s.location).Format("2006-01-02"),
		"days":  days,
		"types": day.Types,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) typesHandler(w http.ResponseWriter, r *http.Request) {
	now, ok := s.resolveNow(w, r.URL.Query())
	if !ok {
		return
	}

	collections, err := s.collections(r.Context())
	if err != nil {
		s.respondUnavailable(w, err)
		return
	}

	todayTypes := today(now, collections, s.location)
	tomorrowTypes := tomorrow(now, collections, s.location)

	resp := map[string]interface{}{
		"today":    todayTypes,
		"tomorrow": tomorrowTypes,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) isTodayHandler(w http.ResponseWriter, r *http.Request) {
	now, ok := s.resolveNow(w, r.URL.Query())
	if !ok {
		return
	}

	collections, err := s.collections(r.Context())
	if err != nil {
		s.respondUnavailable(w, err)
		return
	}

	types := today(now, collections, s.location)
	resp := map[string]interface{}{
		"today": len(types) > 0,
		"types": types,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) isTomorrowHandler(w http.ResponseWriter, r *http.Request) {
	now, ok := s.resolveNow(w, r.URL.Query())
	if !ok {
		return
	}

	collections, err := s.collections(r.Context())
	if err != nil {
		s.respondUnavailable(w, err)
		return
	}

	types := tomorrow(now, collections, s.location)
	resp := map[string]interface{}{
		"tomorrow": len(types) > 0,
		"types":    types,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) collections(ctx context.Context) ([]scraper.Collection, error) {
	if items, ok := s.cache.Get(s.cfg.CacheTTL); ok {
		s.logger.Info("cache hit", slog.Int("items", len(items)))
		return items, nil
	}

	start := time.Now()
	s.logger.Info("scrape start")
	items, err := s.scraper.FetchCollections(ctx)
	if err != nil {
		return nil, err
	}
	s.logger.Info("scrape complete", slog.Int("items", len(items)), slog.Duration("took", time.Since(start)))

	s.cache.Set(items)
	return items, nil
}

func (s *Server) respondScrapeError(w http.ResponseWriter, err error) {
	s.logger.Error("scrape failed", slog.String("error", err.Error()))
	code := http.StatusBadGateway
	detail := "scrape_failed"
	if errors.Is(err, scraper.ErrNoCollections) {
		detail = "failed_to_parse_schedule"
	}
	if errors.Is(err, scraper.ErrAddressSetup) {
		detail = "address_setup_failed"
	}
	writeJSON(w, code, map[string]string{"error": detail})
}

func (s *Server) respondUnavailable(w http.ResponseWriter, err error) {
	s.logger.Error("collections unavailable", slog.String("error", err.Error()))
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
}

func (s *Server) resolveNow(w http.ResponseWriter, values url.Values) (time.Time, bool) {
	now := time.Now().In(s.location)
	input := strings.TrimSpace(values.Get("now"))
	if input == "" {
		return now, true
	}

	parsed, err := time.Parse(time.RFC3339, input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_now"})
		return time.Time{}, false
	}

	return parsed.In(s.location), true
}

func today(now time.Time, collections []scraper.Collection, loc *time.Location) []string {
	for _, day := range groupDays(collections) {
		if sameDay(now, day.Date, loc) && now.Before(day.Date.Add(collectionDuration)) {
			return day.Types
		}
	}
	return []string{}
}

func tomorrow(now time.Time, collections []scraper.Collection, loc *time.Location) []string {
	target := now.AddDate(0, 0, 1)
	for _, day := range groupDays(collections) {
		if sameDay(target, day.Date, loc) {
			return day.Types
		}
	}
	return []string{}
}

func nextDay(now time.Time, collections []scraper.Collection, loc *time.Location) (daySummary, bool) {
	for _, day := range groupDays(collections) {
		if now.Before(day.Date.Add(collectionDuration)) || sameDay(now, day.Date, loc) && !now.After(day.Date.Add(collectionDuration)) {
			return day, true
		}
		if day.Date.After(now) {
			return day, true
		}
	}
	return daySummary{}, false
}

func daysBetween(from, to time.Time, loc *time.Location) int {
	fromDay := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc)
	toDay := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, loc)
	return int(toDay.Sub(fromDay).Hours() / 24)
}

func sameDay(a, b time.Time, loc *time.Location) bool {
	a = a.In(loc)
	b = b.In(loc)
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

type daySummary struct {
	Date  time.Time
	Types []string
}

func groupDays(collections []scraper.Collection) []daySummary {
	cloned := append([]scraper.Collection(nil), collections...)
	sort.Slice(cloned, func(i, j int) bool {
		return cloned[i].Date.Before(cloned[j].Date)
	})

	index := map[string]*daySummary{}
	var keys []string
	for _, c := range cloned {
		key := c.Date.Format("2006-01-02")
		if _, ok := index[key]; !ok {
			index[key] = &daySummary{Date: c.Date}
			keys = append(keys, key)
		}
		if !contains(index[key].Types, c.Type) {
			index[key].Types = append(index[key].Types, c.Type)
		}
	}

	sort.Strings(keys)
	var days []daySummary
	for _, k := range keys {
		days = append(days, *index[k])
	}
	return days
}

func contains(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	data, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, `{"error":"encode_failed"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

type collectionCache struct {
	mu      sync.RWMutex
	items   []scraper.Collection
	fetched time.Time
}

func newCollectionCache() *collectionCache {
	return &collectionCache{}
}

func (c *collectionCache) Get(ttl time.Duration) ([]scraper.Collection, bool) {
	if ttl <= 0 {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.items == nil {
		return nil, false
	}
	if time.Since(c.fetched) > ttl {
		return nil, false
	}
	return append([]scraper.Collection(nil), c.items...), true
}

func (c *collectionCache) Set(items []scraper.Collection) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = append([]scraper.Collection(nil), items...)
	c.fetched = time.Now()
}
