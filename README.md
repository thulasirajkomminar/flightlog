<!-- LOGO -->
<!-- markdownlint-disable MD033 -->
<h1>
<p align="center">
  <img src="./docs/assets/logo.png" alt="Logo" height="200" width="200">
  <br>Flightlog
</p>
</h1>
<!-- markdownlint-enable MD033 -->

A simple app to log and manage your flight history.

## Screenshots

![Login](docs/assets/screenshots/01-login-page.png)

![Dashboard](docs/assets/screenshots/02-dashboard-page.png)

![Search and Add Flight](docs/assets/screenshots/03-search-and-add-flight-page.png)

![My Flights](docs/assets/screenshots/04-my-flights-page.png)

## Getting Started

### Flight Data Provider

Flightlog uses [AeroDataBox](https://aerodatabox.com) via [RapidAPI](https://rapidapi.com/aedbx-aedbx/api/aerodatabox) to fetch flight data. It calls the [Get Flight on Specific Date](https://doc.aerodatabox.com/rapidapi.html#/operations/GetFlight_FlightOnSpecificDate) endpoint and caches results locally in SQLite to minimize API usage.

To get an API key:

1. Sign up at [rapidapi.com](https://rapidapi.com)
2. Subscribe to [AeroDataBox](https://rapidapi.com/aedbx-aedbx/api/aerodatabox) (free tier available)
3. Copy your **X-RapidAPI-Key**

### Configuration

Copy `.env.example` to `.env` and set the required values.

#### Required

| Variable | Description |
| --- | --- |
| `AERODATABOX_API_KEY` | Your RapidAPI key from the step above |
| `AUTH_JWT_SECRET` | Secret for signing JWT tokens (`openssl rand -base64 32`) |

#### Optional

| Variable | Default | Description |
| --- | --- | --- |
| `ENVIRONMENT` | `development` | Set to `production` in prod |
| `SERVER_PORT` | `8080` | HTTP port |
| `DATABASE_PATH` | `data/flightlog.db` | SQLite database path |
| `AUTH_TOKEN_EXPIRY` | `24h` | JWT token lifetime |
| `RATE_LIMIT_IP_REQUESTS_PER_MINUTE` | `100` | Per-IP rate limit |
| `RATE_LIMIT_USER_REQUESTS_PER_MINUTE` | `200` | Per-user rate limit |
| `AERODATABOX_BASE_URL` | `https://aerodatabox.p.rapidapi.com` | API base URL |
| `AERODATABOX_TIMEOUT` | `30s` | API request timeout |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

### Deployment

#### Simple Docker

Create a `docker-compose.yml`:

```yaml
services:
  flightlog:
    image: ghcr.io/thulasirajkomminar/flightlog:latest
    env_file: .env
    ports:
      - 8080:8080
    volumes:
      - flightlog_data:/app/data
    restart: unless-stopped
volumes:
  flightlog_data:
```

```bash
docker compose up -d
```

#### Flightlog with Traefik

```yaml
services:
  flightlog:
    container_name: flightlog
    environment:
      - AERODATABOX_API_KEY=${AERODATABOX_API_KEY}
      - AUTH_JWT_SECRET=${AUTH_JWT_SECRET}
    image: ghcr.io/thulasirajkomminar/flightlog:latest
    labels:
      - traefik.enable=true
      - traefik.http.routers.flightlog.entrypoints=websecure
      - traefik.http.routers.flightlog.rule=Host(`flightlog.example.com`)
      - traefik.http.routers.flightlog.tls=true
      - traefik.http.routers.flightlog.tls.certresolver=cloudflare
      - traefik.http.services.flightlog.loadbalancer.server.port=8080
    networks:
      - proxy
    restart: unless-stopped
    volumes:
      - /opt/docker/flightlog/data:/app/data
networks:
  proxy:
    external: true
```

```bash
docker compose up -d
```

## Import & Export

### Export

Head to **My Flights** and hit the **Export** button — you'll get a CSV file (`flightlog-export.csv`) with everything: flight details, airline info, airport data (with coordinates), times, terminals, gates, aircraft, distance. 29 columns in total.

### Import

From **My Flights**, click **Import** to open the import dialog:

1. **Select source** — pick your CSV format from the dropdown
2. **Upload CSV** — choose your file (up to 5 MB, 100 flights per import)
3. **Preview** — the dialog will tell you how many flights it found
4. **Enrich** *(Flighty only)* — optionally pull in extra detail (gates, aircraft, actual times) for recent flights (within the last year) via AeroDataBox
5. **Import** — you're done

#### Supported Sources

| Source | Description | API Calls |
| --- | --- | --- |
| **Flighty** | Import from a [Flighty](https://www.flightyapp.com) CSV export. These only include the basics (date, airline, flight number, airports), so enrichment is available to fill in the gaps. | Airport lookups + optional enrichment via AeroDataBox |
| **Flightlog** | Re-import your own Flightlog export. All fields come straight from the CSV — no lookups needed. | None |

> [!TIP]
> A handy way to migrate to a new instance: export your flights, then re-import. No API calls required.
