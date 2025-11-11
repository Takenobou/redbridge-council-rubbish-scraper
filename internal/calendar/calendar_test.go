package calendar

import (
	"strings"
	"testing"
	"time"

	"github.com/Takenobou/redbridge-council-rubbish-scraper/internal/scraper"
)

func TestBuilderBuild(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/London")
	b, err := NewBuilder(Config{
		Name:        "Redbridge Collections",
		Description: "Household waste & recycling (scraped)",
		Timezone:    "Europe/London",
	})
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	collections := []scraper.Collection{
		{Date: time.Date(2025, time.December, 2, 6, 0, 0, 0, loc), Type: "Refuse"},
		{Date: time.Date(2025, time.December, 2, 6, 0, 0, 0, loc), Type: "Recycling"},
	}

	data, err := b.Build(collections)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	cal := string(data)

	mustContain(t, cal, "PRODID:-//redbridge-ics//EN")
	mustContain(t, cal, "X-WR-CALNAME:Redbridge Collections")
	mustContain(t, cal, "X-WR-CALDESC:Household waste & recycling (scraped)")
	mustContain(t, cal, "SUMMARY:Bin: Refuse")
	mustContain(t, cal, "SUMMARY:Bin: Recycling")
	mustContain(t, cal, "UID:refuse-20251202@redbridge-ics")
	mustContain(t, cal, "UID:recycling-20251202@redbridge-ics")
	mustContain(t, cal, "TRIGGER:-PT11H")
	mustContain(t, cal, "TRIGGER:-PT30M")
	mustContain(t, cal, "CATEGORIES:Refuse")
	mustContain(t, cal, "CATEGORIES:Recycling")
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected calendar to contain %q", needle)
	}
}
