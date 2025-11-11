package scraper

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var (
	// ErrAddressSetup indicates that the SaveAddress bootstrap call failed.
	ErrAddressSetup = errors.New("failed to seed Redbridge address cookie")
	// ErrNoCollections indicates the scraper could not find any collection slots.
	ErrNoCollections = errors.New("no collections found in schedule")
)

var digitOnly = regexp.MustCompile(`\d+`)

// Config describes how to scrape the council site.
type Config struct {
	BaseURL        string
	SchedulePath   string
	UPRN           string
	AddressLine    string
	Postcode       string
	Latitude       string
	Longitude      string
	UserAgent      string
	StartHour      int
	RequestTimeout time.Duration
	Timezone       string
}

// Collection represents a single waste collection slot.
type Collection struct {
	Date time.Time
	Type string
}

// Scraper performs the SaveAddress handshake and scrapes the upcoming schedule.
type Scraper struct {
	cfg      Config
	location *time.Location
	client   *http.Client
}

// New constructs a Scraper instance.
func New(cfg Config) (*Scraper, error) {
	if cfg.BaseURL == "" || cfg.SchedulePath == "" {
		return nil, errors.New("base URL and schedule path are required")
	}
	if cfg.UPRN == "" {
		return nil, errors.New("UPRN is required")
	}
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone: %w", err)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = 4

	return &Scraper{
		cfg:      cfg,
		location: loc,
		client: &http.Client{
			Timeout:   cfg.RequestTimeout,
			Transport: transport,
		},
	}, nil
}

// FetchCollections scrapes the remote HTML document for upcoming collection dates.
func (s *Scraper) FetchCollections(ctx context.Context) ([]Collection, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	client := *s.client
	client.Jar = jar

	if err := s.seedAddress(ctx, &client); err != nil {
		return nil, err
	}

	// Small pause to avoid hammering the origin immediately.
	select {
	case <-time.After(150 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	body, err := s.fetchSchedule(ctx, &client)
	if err != nil {
		return nil, err
	}

	collections, err := s.parseCollections(body)
	if err != nil {
		return nil, err
	}

	if len(collections) == 0 {
		return nil, ErrNoCollections
	}

	sort.Slice(collections, func(i, j int) bool {
		return collections[i].Date.Before(collections[j].Date)
	})

	return collections, nil
}

func (s *Scraper) seedAddress(ctx context.Context, client *http.Client) error {
	endpoint := fmt.Sprintf("%s/Shared/SaveAddress", s.cfg.BaseURL)
	values := url.Values{}
	values.Set("uprn", s.cfg.UPRN)
	if s.cfg.AddressLine != "" {
		values.Set("address", s.cfg.AddressLine)
	}
	if s.cfg.Postcode != "" {
		values.Set("postcode", s.cfg.Postcode)
	}
	if s.cfg.Latitude != "" {
		values.Set("latitude", s.cfg.Latitude)
	}
	if s.cfg.Longitude != "" {
		values.Set("longitude", s.cfg.Longitude)
	}
	values.Set("_", fmt.Sprintf("%d", time.Now().UnixMilli()))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+values.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", s.cfg.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("save address: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("%w: status %d", ErrAddressSetup, resp.StatusCode)
	}

	hasCookie := false
	for _, c := range resp.Cookies() {
		if c.Name == "RedbridgeIV3LivePref" {
			hasCookie = true
			break
		}
	}
	if !hasCookie {
		// If the cookie is already stored, the response may omit it. Accept that scenario.
		cookies := client.Jar.Cookies(req.URL)
		for _, c := range cookies {
			if c.Name == "RedbridgeIV3LivePref" {
				hasCookie = true
				break
			}
		}
	}
	if !hasCookie {
		return ErrAddressSetup
	}

	return nil
}

func (s *Scraper) fetchSchedule(ctx context.Context, client *http.Client) ([]byte, error) {
	endpoint := fmt.Sprintf("%s%s", s.cfg.BaseURL, s.cfg.SchedulePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", s.cfg.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch schedule: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("fetch schedule: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (s *Scraper) parseCollections(body []byte) ([]Collection, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	container := doc.Find(".your-collection-schedule-container").First()
	if container.Length() == 0 {
		return nil, ErrNoCollections
	}

	defs := []blockDefinition{
		{
			blockSelector: ".refuse-container",
			entrySelector: ".collectionDates-container .garden-collection-postdate",
			daySelector:   ".refuse-garden-collection-day-numeric",
			monthSelector: ".refuse-collection-month",
			wasteType:     "Refuse",
		},
		{
			blockSelector: ".recycle-container",
			entrySelector: ".collectionDates-container .garden-collection-postdate",
			daySelector:   ".recycling-garden-collection-day-numeric",
			monthSelector: ".recycling-collection-month",
			wasteType:     "Recycling",
		},
		{
			blockSelector: ".garden-container",
			entrySelector: ".collectionDates-container .garden-collection-postdate",
			daySelector:   ".garden-collection-day-numeric",
			monthSelector: ".garden-collection-month",
			wasteType:     "Garden Waste",
		},
	}

	var results []Collection
	seen := make(map[string]struct{})

	for _, def := range defs {
		block := container.Find(def.blockSelector)
		if block.Length() == 0 {
			continue
		}
		block.Find(def.entrySelector).Each(func(_ int, sel *goquery.Selection) {
			dayText := strings.TrimSpace(sel.Find(def.daySelector).Text())
			monthText := strings.TrimSpace(sel.Find(def.monthSelector).Text())
			if dayText == "" || monthText == "" {
				return
			}

			date, err := s.parseDate(dayText, monthText)
			if err != nil {
				return
			}

			key := fmt.Sprintf("%s|%s", date.Format(time.RFC3339), def.wasteType)
			if _, exists := seen[key]; exists {
				return
			}
			seen[key] = struct{}{}

			results = append(results, Collection{
				Date: date,
				Type: def.wasteType,
			})
		})
	}

	return results, nil
}

func (s *Scraper) parseDate(dayText, monthText string) (time.Time, error) {
	dayDigits := digitOnly.FindString(dayText)
	if dayDigits == "" {
		return time.Time{}, errors.New("invalid day")
	}

	monthClean := normalizeSpaces(monthText)
	if monthClean == "" {
		return time.Time{}, errors.New("invalid month")
	}

	full := fmt.Sprintf("%s %s", dayDigits, monthClean)
	parsed, err := time.ParseInLocation("2 January 2006", full, s.location)
	if err != nil {
		return time.Time{}, err
	}

	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), s.cfg.StartHour, 0, 0, 0, s.location), nil
}

type blockDefinition struct {
	blockSelector string
	entrySelector string
	daySelector   string
	monthSelector string
	wasteType     string
}

func normalizeSpaces(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
