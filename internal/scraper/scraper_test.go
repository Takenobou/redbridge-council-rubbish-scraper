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
	var gardenNote string
	var foodInstructions []Instruction
	for _, c := range collections {
		if c.Type == "Garden Waste" {
			foundGarden++
			gardenNote = c.Note
		}
		if c.Type == "Food Waste" {
			foundFood++
			if len(foodInstructions) == 0 {
				foodInstructions = c.Instructions
			}
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
	if gardenNote != "Date changed due to bank holiday." {
		t.Fatalf("expected garden note, got %q", gardenNote)
	}
	if len(foodInstructions) != 3 {
		t.Fatalf("expected 3 food instructions, got %d", len(foodInstructions))
	}
	expectedLink := ts.URL + "/MissedCollection/foodwaste"
	if foodInstructions[0].Text != "Please place your outside food waste caddy at the boundary of your property by 6.00am on your collection day." {
		t.Fatalf("unexpected food instruction 1: %q", foodInstructions[0].Text)
	}
	if len(foodInstructions[0].Links) != 0 {
		t.Fatalf("unexpected food instruction 1 links: %v", foodInstructions[0].Links)
	}
	if foodInstructions[1].Text != "Please put the handle of your caddy into locked position to prevent pests." {
		t.Fatalf("unexpected food instruction 2: %q", foodInstructions[1].Text)
	}
	if len(foodInstructions[1].Links) != 0 {
		t.Fatalf("unexpected food instruction 2 links: %v", foodInstructions[1].Links)
	}
	if foodInstructions[2].Text != "Missed collection? Report missed food waste collection" {
		t.Fatalf("unexpected food instruction 3: %q", foodInstructions[2].Text)
	}
	if len(foodInstructions[2].Links) != 1 || foodInstructions[2].Links[0] != expectedLink {
		t.Fatalf("unexpected food instruction 3 links: %v", foodInstructions[2].Links)
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

func TestFetchCollectionsGardenNotice(t *testing.T) {
	html := loadFixture(t, "testdata/schedule_garden_missing.html")
	notice := "The fortnightly Garden Waste Collection Service will resume in the Spring"

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
	if len(collections) != 2 {
		t.Fatalf("expected 2 collections, got %d", len(collections))
	}
	for _, c := range collections {
		if c.Type == "Garden Waste" {
			t.Fatalf("did not expect garden waste collections")
		}
		if !strings.Contains(c.Note, notice) {
			t.Fatalf("expected garden notice in %s note, got %q", c.Type, c.Note)
		}
	}
}

func TestParseCollectionsCurrentGardenMarkup(t *testing.T) {
	html := `<div class="your-collection-schedule-container">
  <div class="garden-container">
    <div class="collectionType">
      <h3>Garden Waste</h3>
      <p></p>
    </div>
    <p class="upcoming-dates">Upcoming collection dates:</p>
    <div class="collectionDates-container bs3-col-sm-12">
      <div class="garden-collection-postdate bs3-col-sm-2">
        <div class="garden-collection-day-of-week">Wednesday</div>
        <div class="garden-garden-collection-day-numeric">24</div>
        <div class="garden-collection-month">June 2026</div>
      </div>
      <div class="garden-collection-postdate bs3-col-sm-2">
        <div class="garden-collection-day-of-week">Wednesday</div>
        <div class="garden-garden-collection-day-numeric">08</div>
        <div class="garden-collection-month">July 2026</div>
      </div>
      <div class="garden-collection-postdate bs3-col-sm-2">
        <div class="garden-collection-day-of-week">Wednesday</div>
        <div class="garden-garden-collection-day-numeric">22</div>
        <div class="garden-collection-month">July 2026</div>
      </div>
      <div class="garden-collection-postdate bs3-col-sm-2">
        <div class="garden-collection-day-of-week">Wednesday</div>
        <div class="garden-garden-collection-day-numeric">05</div>
        <div class="garden-collection-month">August 2026</div>
      </div>
      <div class="garden-collection-postdate bs3-col-sm-2">
        <div class="garden-collection-day-of-week">Wednesday</div>
        <div class="garden-garden-collection-day-numeric">19</div>
        <div class="garden-collection-month">August 2026</div>
      </div>
    </div>
    <div class="collectionDetail bs3-col-sm-12">
      <p class="instructions smalltext muted">
        Please place your garden waste at the boundary of your property by <strong>6.00am</strong>.<br>
        We will collect up to <strong>5 sacks.</strong> Leave all sacks open so the waste can be seen.<br>
        You can use Green Garden Waste sacks, black bin bags or clear sacks.
      </p>
      <p class="instructions smalltext muted">
        <span class="missed-text">Missed collection?</span>
        <a href="/MissedCollection/garden" class="redbridge-link">Report missed garden waste collection</a>
      </p>
    </div>
  </div>
</div>`

	s, err := New(Config{
		BaseURL:        "https://www.redbridge.gov.uk",
		SchedulePath:   "/RecycleRefuse",
		UPRN:           "123",
		StartHour:      6,
		RequestTimeout: time.Second,
		Timezone:       "Europe/London",
	})
	if err != nil {
		t.Fatalf("New scraper: %v", err)
	}

	collections, err := s.parseCollections([]byte(html))
	if err != nil {
		t.Fatalf("parseCollections: %v", err)
	}
	if len(collections) != 5 {
		t.Fatalf("expected 5 garden collections, got %d", len(collections))
	}

	first := collections[0]
	if first.Type != "Garden Waste" {
		t.Fatalf("expected Garden Waste, got %s", first.Type)
	}
	expectedDate := time.Date(2026, time.June, 24, 6, 0, 0, 0, s.location)
	if !first.Date.Equal(expectedDate) {
		t.Fatalf("expected first date %s, got %s", expectedDate, first.Date)
	}
	if len(first.Instructions) != 2 {
		t.Fatalf("expected 2 garden instructions, got %d", len(first.Instructions))
	}
	expectedLink := "https://www.redbridge.gov.uk/MissedCollection/garden"
	if len(first.Instructions[1].Links) != 1 || first.Instructions[1].Links[0] != expectedLink {
		t.Fatalf("unexpected garden missed collection link: %v", first.Instructions[1].Links)
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
