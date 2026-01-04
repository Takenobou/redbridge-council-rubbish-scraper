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
	productID          = "-//redbridge-ics//EN"
	defaultInstruction = "Place bins out by 06:00 on collection day."
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
		event.SetDescription(eventDescription(collection))
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

func eventDescription(collection scraper.Collection) string {
	instructionTexts, missedLinks, otherLinks := splitInstructions(collection.Instructions)
	if len(instructionTexts) == 0 {
		instructionTexts = []string{defaultInstruction}
	}

	var sections []string
	if section := formatInstructionSection(instructionTexts); section != "" {
		sections = append(sections, section)
	}
	if len(missedLinks) > 0 {
		sections = append(sections, formatLinksSection("MISSED COLLECTION", missedLinks))
	}
	if len(otherLinks) > 0 {
		sections = append(sections, formatLinksSection("LINKS", otherLinks))
	}
	if note := strings.TrimSpace(collection.Note); note != "" {
		sections = append(sections, formatNoteSection(note))
	}

	return strings.Join(sections, "\n\n")
}

func splitInstructions(lines []scraper.Instruction) ([]string, []string, []string) {
	var instructionTexts []string
	var missedLinks []string
	var otherLinks []string

	for _, line := range lines {
		text := strings.TrimSpace(line.Text)
		links := cleanLinks(line.Links)
		if len(links) == 0 {
			if text != "" {
				instructionTexts = append(instructionTexts, text)
			}
			continue
		}
		if isMissedCollection(text, links) {
			missedLinks = appendUnique(missedLinks, links...)
			continue
		}
		if text != "" {
			instructionTexts = append(instructionTexts, text)
		}
		otherLinks = appendUnique(otherLinks, links...)
	}

	return instructionTexts, missedLinks, otherLinks
}

func cleanLinks(links []string) []string {
	var cleaned []string
	for _, link := range links {
		link = strings.TrimSpace(link)
		if link == "" {
			continue
		}
		cleaned = append(cleaned, link)
	}
	return cleaned
}

func appendUnique(dst []string, values ...string) []string {
	seen := make(map[string]struct{}, len(dst))
	for _, v := range dst {
		seen[v] = struct{}{}
	}
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		dst = append(dst, v)
	}
	return dst
}

func isMissedCollection(text string, links []string) bool {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "missed collection") {
		return true
	}
	for _, link := range links {
		if strings.Contains(strings.ToLower(link), "/missedcollection") {
			return true
		}
	}
	return false
}

func formatInstructionSection(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("INSTRUCTIONS")
	for _, line := range lines {
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		fmt.Fprintf(&b, "\n• %s", text)
	}
	return b.String()
}

func formatLinksSection(title string, links []string) string {
	if len(links) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(title)
	for _, link := range links {
		link = strings.TrimSpace(link)
		if link == "" {
			continue
		}
		b.WriteString("\n")
		b.WriteString(link)
	}
	return b.String()
}

func formatNoteSection(note string) string {
	lines := strings.Split(note, "\n")
	var b strings.Builder
	b.WriteString("NOTE")
	for _, line := range lines {
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		fmt.Fprintf(&b, "\n• %s", text)
	}
	return b.String()
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
