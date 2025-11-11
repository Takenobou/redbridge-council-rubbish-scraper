package calendar

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	ics "github.com/arran4/golang-ical"

	"github.com/Takenobou/redbridge-council-rubbish-scraper/internal/scraper"
)

const (
	productID            = "-//redbridge-ics//EN"
	eventDescriptionBody = "Place bins out by 06:00 on collection day."
)

var slugRegex = regexp.MustCompile(`[^a-z0-9]+`)

// Config defines calendar level metadata.
type Config struct {
	Name        string
	Description string
	Timezone    string
}

// Builder transforms scraped data into an .ics payload.
type Builder struct {
	cfg      Config
	location *time.Location
}

// NewBuilder initialises a calendar builder with timezone handling.
func NewBuilder(cfg Config) (*Builder, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("calendar name is required")
	}
	if cfg.Timezone == "" {
		cfg.Timezone = "Europe/London"
	}

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone: %w", err)
	}

	return &Builder{
		cfg:      cfg,
		location: loc,
	}, nil
}

// Build creates the textual iCalendar representation.
func (b *Builder) Build(collections []scraper.Collection) ([]byte, error) {
	cal := ics.NewCalendar()
	cal.SetProductId(productID)
	cal.SetCalscale("GREGORIAN")
	cal.SetMethod(ics.MethodPublish)
	cal.SetName(b.cfg.Name)
	if b.cfg.Description != "" {
		cal.SetDescription(b.cfg.Description)
		cal.SetXWRCalDesc(b.cfg.Description)
	}

	for _, collection := range collections {
		event := cal.AddEvent(eventID(collection))
		event.SetSummary(fmt.Sprintf("Bin: %s", titleCase(collection.Type)))
		event.SetDescription(eventDescriptionBody)
		event.SetProperty(ics.ComponentPropertyCategories, collection.Type)

		start := collection.Date.In(b.location)
		end := start.Add(time.Hour)
		event.SetStartAt(start)
		event.SetEndAt(end)
		event.SetDtStampTime(time.Now())

		addAlarm(event, "-PT11H")
		addAlarm(event, "-PT30M")
	}

	return []byte(cal.Serialize()), nil
}

func addAlarm(event *ics.VEvent, trigger string) {
	alarm := event.AddAlarm()
	alarm.SetAction(ics.ActionDisplay)
	alarm.SetDescription("Bin reminder")
	alarm.SetTrigger(trigger)
}

func eventID(collection scraper.Collection) string {
	date := collection.Date.Format("20060102")
	return fmt.Sprintf("%s-%s@redbridge-ics", slug(collection.Type), date)
}

func slug(value string) string {
	lower := strings.ToLower(value)
	lower = slugRegex.ReplaceAllString(lower, "-")
	return strings.Trim(lower, "-")
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
