# Redbridge Council Rubbish Scraper

Minimal Go microservice that emulates the Redbridge “SaveAddress” handshake, scrapes the council’s bin schedule, and publishes the dates as an `.ics` feed plus lightweight JSON endpoints you can plug into automations.

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

## Docker quick start

Pull the image hosted at `ghcr.io/takenobou/redbridge-council-rubbish-scraper` and supply your address details:

```bash
docker run -d \
  --name redbridge-ics \
  -p 8080:8080 \
  -e UPRN="YOUR_UPRN" \
  -e ADDRESS_LINE="123 SAMPLE STREET" \
  -e POSTCODE="IG1 1AA" \
  ghcr.io/takenobou/redbridge-council-rubbish-scraper:latest
```

For docker-compose / stacks, drop this into your compose file:

```yaml
services:
  redbridge-ics:
    image: ghcr.io/takenobou/redbridge-council-rubbish-scraper:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      UPRN: "YOUR_UPRN"
      ADDRESS_LINE: "123 SAMPLE STREET"
      POSTCODE: "IG1 1AA"
      CACHE_TTL: "168h"
```

Both examples expose the service on port 8080; point Apple Calendar or Home Assistant at `http://<host>:8080/calendar.ics`.