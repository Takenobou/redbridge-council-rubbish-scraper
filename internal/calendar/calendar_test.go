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
		{Date: time.Date(2025, time.December, 2, 6, 0, 0, 0, loc), Type: "Refuse", Note: "Date changed due to bank holiday."},
		{Date: time.Date(2025, time.December, 2, 6, 0, 0, 0, loc), Type: "Recycling", Instructions: []scraper.Instruction{
			{Text: "Rinse containers before recycling."},
			{Text: "Missed collection? Report missed recycling collection", Links: []string{"https://my.redbridge.gov.uk/MissedCollection/recycling"}},
		}},
	}

	data, err := b.Build(collections)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	cal := unfoldICS(string(data))

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
	mustContain(t, cal, "INSTRUCTIONS")
	mustContain(t, cal, "• Place bins out by 06:00 on collection day.")
	mustContain(t, cal, "NOTE")
	mustContain(t, cal, "• Date changed due to bank holiday.")
	mustContain(t, cal, "• Rinse containers before recycling.")
	mustContain(t, cal, "MISSED COLLECTION")
	mustContain(t, cal, "https://my.redbridge.gov.uk/MissedCollection/recycling")
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected calendar to contain %q", needle)
	}
}

func unfoldICS(value string) string {
	value = strings.ReplaceAll(value, "\r\n ", "")
	value = strings.ReplaceAll(value, "\n ", "")
	return value
}
