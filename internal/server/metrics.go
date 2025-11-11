package server

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type metrics struct {
	registry       *prometheus.Registry
	cacheHits      prometheus.Counter
	cacheMisses    prometheus.Counter
	scrapeRequests prometheus.Counter
	scrapeFailures prometheus.Counter
	scrapeDuration prometheus.Histogram
	lastScrapeTime prometheus.Gauge
}

func newMetrics() *metrics {
	reg := prometheus.NewRegistry()

	m := &metrics{
		registry: reg,
		cacheHits: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "redbridge_cache_hits_total",
			Help: "Number of times collections were served from cache",
		}),
		cacheMisses: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "redbridge_cache_misses_total",
			Help: "Number of times cache was cold or expired",
		}),
		scrapeRequests: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "redbridge_scrapes_total",
			Help: "Number of scrape attempts against Redbridge",
		}),
		scrapeFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "redbridge_scrape_failures_total",
			Help: "Number of scrape attempts that failed",
		}),
		scrapeDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "redbridge_scrape_duration_seconds",
			Help:    "Time taken to perform a full scrape",
			Buckets: prometheus.DefBuckets,
		}),
		lastScrapeTime: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "redbridge_last_scrape_timestamp_seconds",
			Help: "Unix timestamp of the last successful scrape",
		}),
	}

	reg.MustRegister(
		m.cacheHits,
		m.cacheMisses,
		m.scrapeRequests,
		m.scrapeFailures,
		m.scrapeDuration,
		m.lastScrapeTime,
	)

	return m
}

func (m *metrics) handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
