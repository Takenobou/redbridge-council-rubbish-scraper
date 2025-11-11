package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Takenobou/redbridge-council-rubbish-scraper/internal/calendar"
	"github.com/Takenobou/redbridge-council-rubbish-scraper/internal/config"
	"github.com/Takenobou/redbridge-council-rubbish-scraper/internal/scraper"
)

func TestCollectionsCache(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(httptest.NewRecorder(), nil))

	s := &fakeScraper{
		collections: []scraper.Collection{
			{Date: mustDate(t, 2025, 12, 2, 6), Type: "Refuse"},
		},
	}
	cal, _ := calendar.NewBuilder(calendar.Config{
		Name:     "Redbridge Collections",
		Timezone: "Europe/London",
	})

	cfg := config.Config{
		ListenAddr: ":0",
		CacheTTL:   time.Hour,
		Timezone:   "Europe/London",
	}

	srv := New(cfg, s, cal, logger)

	if _, err := srv.collections(context.Background(), false); err != nil {
		t.Fatalf("collections: %v", err)
	}
	if _, err := srv.collections(context.Background(), false); err != nil {
		t.Fatalf("collections: %v", err)
	}
	if s.calls != 1 {
		t.Fatalf("expected cache hit, scraper called %d times", s.calls)
	}

	if _, err := srv.collections(context.Background(), true); err != nil {
		t.Fatalf("force collections: %v", err)
	}
	if s.calls != 2 {
		t.Fatalf("expected force refresh to call scraper again, got %d", s.calls)
	}
}

func TestNextHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(httptest.NewRecorder(), nil))

	cals := &noopCalendar{}
	s := &fakeScraper{
		collections: []scraper.Collection{
			{Date: mustDate(t, 2025, 12, 1, 6), Type: "Refuse"},
			{Date: mustDate(t, 2025, 12, 2, 6), Type: "Recycling"},
		},
	}

	cfg := config.Config{
		ListenAddr: ":0",
		CacheTTL:   time.Hour,
		Timezone:   "Europe/London",
	}

	srv := New(cfg, s, cals, logger)

	req := httptest.NewRequest("GET", "/api/next?now=2025-12-01T07:30:00Z", nil)
	rr := httptest.NewRecorder()
	srv.nextHandler(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var payload struct {
		Date  string   `json:"date"`
		Days  int      `json:"days"`
		Types []string `json:"types"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if payload.Date != "2025-12-02" {
		t.Fatalf("expected next date 2025-12-02, got %s", payload.Date)
	}
	if payload.Days != 1 {
		t.Fatalf("expected days 1, got %d", payload.Days)
	}
	if len(payload.Types) != 1 || payload.Types[0] != "Recycling" {
		t.Fatalf("unexpected types %v", payload.Types)
	}
}

func TestCalendarHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(httptest.NewRecorder(), nil))
	s := &fakeScraper{
		collections: []scraper.Collection{
			{Date: mustDate(t, 2025, 12, 1, 6), Type: "Refuse"},
		},
	}
	cal := &fakeCalendarBuilder{ics: []byte("BEGIN:VCALENDAR\nEND:VCALENDAR")}
	cfg := config.Config{
		ListenAddr: ":0",
		CacheTTL:   time.Hour,
		Timezone:   "Europe/London",
	}
	srv := New(cfg, s, cal, logger)

	req := httptest.NewRequest("GET", "/calendar.ics", nil)
	rr := httptest.NewRecorder()
	srv.calendarHandler(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "text/calendar; charset=utf-8" {
		t.Fatalf("unexpected content-type %s", got)
	}
	if body := rr.Body.String(); !strings.Contains(body, "VCALENDAR") {
		t.Fatalf("unexpected body %s", body)
	}
}

type fakeScraper struct {
	collections []scraper.Collection
	err         error
	calls       int
}

func (f *fakeScraper) FetchCollections(ctx context.Context) ([]scraper.Collection, error) {
	f.calls++
	return f.collections, f.err
}

type fakeCalendarBuilder struct {
	ics []byte
	err error
}

func (f *fakeCalendarBuilder) Build(collections []scraper.Collection) ([]byte, error) {
	return f.ics, f.err
}

type noopCalendar struct{}

func (n *noopCalendar) Build(collections []scraper.Collection) ([]byte, error) {
	return []byte(""), nil
}

func mustDate(t *testing.T, year int, month time.Month, day, hour int) time.Time {
	t.Helper()
	loc, _ := time.LoadLocation("Europe/London")
	return time.Date(year, month, day, hour, 0, 0, 0, loc)
}
