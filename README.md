# Redbridge Council Rubbish Scraper

Tiny Go HTTP service that pretends to be the Redbridge “SaveAddress” flow once a week, scrapes the five-week schedule, and exposes both an `.ics` feed (with VALARMs) and JSON helpers you can plug into automations.

## Project layout

```
cmd/api            # 12-factor entrypoint
internal/config    # Environment-driven runtime config
internal/scraper   # SaveAddress bootstrap + goquery parser
internal/calendar  # arran4/golang-ical builder with alarms
internal/server    # net/http handlers, caching, date helpers
```

## HTTP surface

- `GET /calendar.ics` – ICS feed with `PRODID:-//redbridge-ics//EN`, per-type events at 06:00–07:00, and two `VALARM`s (`-PT11H`, `-PT30M`).
- `GET /api/next` – `{ "date":"2025-11-11","days":0,"types":["Refuse","Recycling"] }`, skips the current day after 07:00.
- `GET /api/types` – `{ "today":[...], "tomorrow":[...] }`.
- `GET /api/is-today` / `GET /api/is-tomorrow` – boolean + `types` array payloads.
- `GET /healthz` – liveness check.
- `GET /metrics` – Prometheus metrics (cache hits/misses, scrape timings).

JSON endpoints support `?now=YYYY-MM-DDTHH:MM:SS±HH:MM` overrides for deterministic tests, and the server automatically re-scrapes whenever the cached data expires.

## Configuration

| Variable | Description | Default |
| --- | --- | --- |
| `LISTEN_ADDR` | HTTP bind address | `:8080` |
| `BASE_URL` | Redbridge root URL | `https://my.redbridge.gov.uk` |
| `SCHEDULE_PATH` | Path to the recycle/refuse page | `/RecycleRefuse` |
| `UPRN` | Required UPRN used in `SaveAddress` | **required** |
| `ADDRESS_LINE` | Optional address line | – |
| `POSTCODE` | Optional postcode | – |
| `LATITUDE`/`LONGITUDE` | Optional coordinates | – |
| `CACHE_TTL` | Go duration for collection cache | `168h` |
| `START_HOUR` | Hour (24h) to schedule events | `6` |
| `USER_AGENT` | HTTP User-Agent for both requests | `redbridge-council-rubbish-scraper/1.0` |
| `SCRAPE_TIMEOUT` | HTTP timeout for SaveAddress + fetch | `15s` |

Timezone is fixed to `Europe/London` so “today/tomorrow” calculations align with council advice. Set `CACHE_TTL` to match however often you want to re-scrape (weekly by default).

## Running locally

```bash
go run ./cmd/api
```

Visit `http://localhost:8080/calendar.ics` to prime the cache (first hit scrapes), or `.../api/next` to exercise the JSON logic during tests.

## Future tests

- Fixture-driven parser test with saved HTML markup to guard selectors.
- Time-travel unit tests for the “after 07:00” edge cases and DST transitions.
- Golden snapshot asserting ICS headers, event ordering, and `VALARM`s.

Hook it up to Home Assistant or subscribe in Apple Calendar via `https://your-domain/calendar.ics` once you’ve put it behind HTTPS.
