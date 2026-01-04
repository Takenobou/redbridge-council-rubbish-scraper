package scraper

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFetchCollectionsSuccess(t *testing.T) {
	html := loadFixture(t, "testdata/schedule.html")

	mux := http.NewServeMux()
	mux.HandleFunc("/Shared/SaveAddress", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "RedbridgeIV3LivePref", Value: "abc"})
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/RecycleRefuse", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	s, err := New(Config{
		BaseURL:        ts.URL,
		SchedulePath:   "/RecycleRefuse",
		UPRN:           "123",
		UserAgent:      "test-agent",
		StartHour:      6,
		RequestTimeout: time.Second,
		Timezone:       "Europe/London",
	})
	if err != nil {
		t.Fatalf("New scraper: %v", err)
	}
	s.client = ts.Client()

	collections, err := s.FetchCollections(context.Background())
	if err != nil {
		t.Fatalf("FetchCollections: %v", err)
	}

	if len(collections) != 7 {
		t.Fatalf("expected 7 collections, got %d", len(collections))
	}

	first := collections[0]
	if got := first.Date.Hour(); got != 6 {
		t.Fatalf("expected start hour 6, got %d", got)
	}
	if first.Type != "Refuse" {
		t.Fatalf("expected first type Refuse, got %s", first.Type)
	}

	foundGarden := 0
	foundFood := 0
	for _, c := range collections {
		if c.Type == "Garden Waste" {
			foundGarden++
		}
		if c.Type == "Food Waste" {
			foundFood++
		}
		if c.Date.Location().String() != "Europe/London" {
			t.Fatalf("date in wrong location: %s", c.Date.Location())
		}
	}
	if foundGarden != 1 {
		t.Fatalf("expected dedup garden to 1, got %d", foundGarden)
	}
	if foundFood != 2 {
		t.Fatalf("expected food waste entries to 2, got %d", foundFood)
	}
}

func TestFetchCollectionsSaveAddressFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/Shared/SaveAddress", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	s, err := New(Config{
		BaseURL:        ts.URL,
		SchedulePath:   "/RecycleRefuse",
		UPRN:           "123",
		UserAgent:      "test-agent",
		StartHour:      6,
		RequestTimeout: time.Second,
		Timezone:       "Europe/London",
	})
	if err != nil {
		t.Fatalf("New scraper: %v", err)
	}
	s.client = ts.Client()

	_, err = s.FetchCollections(context.Background())
	if err == nil || !strings.Contains(err.Error(), "address") {
		t.Fatalf("expected address error, got %v", err)
	}
}

func TestFetchCollectionsSaveAddressFailureWithCookie(t *testing.T) {
	html := loadFixture(t, "testdata/schedule.html")

	mux := http.NewServeMux()
	mux.HandleFunc("/Shared/SaveAddress", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "RedbridgeIV3LivePref", Value: "abc"})
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/RecycleRefuse", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	s, err := New(Config{
		BaseURL:        ts.URL,
		SchedulePath:   "/RecycleRefuse",
		UPRN:           "123",
		UserAgent:      "test-agent",
		StartHour:      6,
		RequestTimeout: time.Second,
		Timezone:       "Europe/London",
	})
	if err != nil {
		t.Fatalf("New scraper: %v", err)
	}
	s.client = ts.Client()

	if _, err := s.FetchCollections(context.Background()); err != nil {
		t.Fatalf("FetchCollections: %v", err)
	}
}

func TestFetchCollectionsNoCollections(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/Shared/SaveAddress", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "RedbridgeIV3LivePref", Value: "abc"})
	})
	mux.HandleFunc("/RecycleRefuse", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	s, err := New(Config{
		BaseURL:        ts.URL,
		SchedulePath:   "/RecycleRefuse",
		UPRN:           "123",
		UserAgent:      "test-agent",
		StartHour:      6,
		RequestTimeout: time.Second,
		Timezone:       "Europe/London",
	})
	if err != nil {
		t.Fatalf("New scraper: %v", err)
	}
	s.client = ts.Client()

	_, err = s.FetchCollections(context.Background())
	if !errors.Is(err, ErrNoCollections) {
		t.Fatalf("expected ErrNoCollections, got %v", err)
	}
}

func loadFixture(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(data)
}
