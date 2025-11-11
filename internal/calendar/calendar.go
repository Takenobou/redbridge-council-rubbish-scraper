package calendar

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	ics "github.com/arran4/golang-ical"

	"github.com/Takenobou/redbridge-council-rubbish-scraper/internal/scraper"
)

// Builder transforms scraped data into an .ics payload.
type Builder struct {
	serviceName string
	location    *time.Location
}

// NewBuilder initialises a calendar builder with timezone handling.
func NewBuilder(serviceName, timezone string) (*Builder, error) {
	if serviceName == "" {
		return nil, fmt.Errorf("service name is required")
	}

	if timezone == "" {
		timezone = "Europe/London"
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone: %w", err)
	}

	return &Builder{
		serviceName: serviceName,
		location:    loc,
	}, nil
}

// Build creates the textual iCalendar representation.
func (b *Builder) Build(collections []scraper.Collection) ([]byte, error) {
	cal := ics.NewCalendar()
	cal.SetProductId("-//redbridge council scraper//EN")
	cal.SetCalscale("GREGORIAN")
	cal.SetMethod(ics.MethodPublish)
	cal.SetName(b.serviceName)

	for _, collection := range collections {
		event := cal.AddEvent(eventID(collection))
		event.SetSummary(fmt.Sprintf("%s collection", titleCase(collection.Type)))
		event.SetDescription("Scheduled waste collection provided by Redbridge Council")

		start := collection.Date.In(b.location)
		end := start.Add(time.Hour)
		event.SetStartAt(start)
		event.SetEndAt(end)
		event.SetAllDayStartAt(start)
		event.SetAllDayEndAt(end)
	}

	return []byte(cal.Serialize()), nil
}

func eventID(collection scraper.Collection) string {
	hash := sha1.Sum([]byte(collection.Date.Format(time.RFC3339) + collection.Type))
	return fmt.Sprintf("collection-%s@redbridge.local", hex.EncodeToString(hash[:]))
}

func titleCase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Collection"
	}

	words := strings.Fields(value)
	for i, w := range words {
		r, size := utf8.DecodeRuneInString(w)
		if r == utf8.RuneError && size == 0 {
			continue
		}

		words[i] = strings.ToUpper(string(r)) + strings.ToLower(w[size:])
	}

	return strings.Join(words, " ")
}
