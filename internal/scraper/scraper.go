package scraper

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
)

// Config describes how to scrape the council site.
type Config struct {
	ScheduleURL        string
	CollectionSelector string
	DateSelector       string
	TypeSelector       string
	UserAgent          string
}

// Collection represents a single waste collection slot.
type Collection struct {
	Date time.Time
	Type string
}

// Scraper wraps a configured Colly collector.
type Scraper struct {
	cfg       Config
	collector *colly.Collector
}

// New constructs a Scraper instance.
func New(cfg Config) (*Scraper, error) {
	if cfg.ScheduleURL == "" {
		return nil, errors.New("schedule URL cannot be empty")
	}

	c := colly.NewCollector()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}

	return &Scraper{
		cfg:       cfg,
		collector: c,
	}, nil
}

// FetchCollections scrapes the remote HTML document for upcoming collection dates.
func (s *Scraper) FetchCollections(ctx context.Context) ([]Collection, error) {
	if ctx == nil {
		return nil, errors.New("context cannot be nil")
	}

	var (
		mu          sync.Mutex
		results     []Collection
		scrapeError error
	)

	collector := s.collector.Clone()

	collector.OnRequest(func(r *colly.Request) {
		if err := ctx.Err(); err != nil {
			r.Abort()
		}
	})

	collector.OnError(func(_ *colly.Response, err error) {
		scrapeError = err
	})

	collector.OnHTML(s.cfg.CollectionSelector, func(e *colly.HTMLElement) {
		dateRaw := strings.TrimSpace(e.ChildText(s.cfg.DateSelector))
		typeRaw := strings.TrimSpace(e.ChildText(s.cfg.TypeSelector))
		if dateRaw == "" || typeRaw == "" {
			return
		}

		dateParsed, err := parseDate(dateRaw)
		if err != nil {
			return
		}

		mu.Lock()
		results = append(results, Collection{
			Date: dateParsed,
			Type: typeRaw,
		})
		mu.Unlock()
	})

	if err := collector.Visit(s.cfg.ScheduleURL); err != nil {
		return nil, err
	}

	if scrapeError != nil {
		return nil, scrapeError
	}

	return results, ctx.Err()
}

var dateLayouts = []string{
	time.RFC3339,
	"2006-01-02",
	"02-01-2006",
	"02/01/2006",
	"2 January 2006",
	"02 January 2006",
	"Monday 02 January 2006",
	"02 Jan 2006",
}

func parseDate(value string) (time.Time, error) {
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse date %q", value)
}
