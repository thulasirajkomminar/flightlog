# Getting started

Flightlog ships as a single Docker image. If you can run `docker compose up`, you can run Flightlog.

This page walks you through the minimum needed to get a working instance — one container, one volume, one browser tab.

## Before you start

You'll need:

- **Docker** (or Docker Desktop) installed on the machine that will host Flightlog
- **An AeroDataBox API key** — free tier is plenty for a personal logbook. [Grab one in two minutes :material-arrow-right:](api-key.md)
- **A JWT secret** — used to sign your login session. Generate one with:

  ```bash
  openssl rand -base64 32
  ```

That's it. No database server to install, no Redis, nothing else. Flightlog keeps everything in a single SQLite file on disk.

## 1. Create an `.env` file

Drop the API key and JWT secret into a file called `.env`:

```bash title=".env"
AERODATABOX_API_KEY=your_rapidapi_key_here
AUTH_JWT_SECRET=paste_the_openssl_output_here
```

A couple of optional knobs you can add — see [Configuration](configuration.md) for the full list:

```bash
LOG_LEVEL=info          # debug | info | warn | error
SERVER_PORT=8080        # port Flightlog listens on
```

## 2. Create `docker-compose.yml`

```yaml title="docker-compose.yml"
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

The `flightlog_data` volume holds the SQLite database — back this up and your entire history is portable.

## 3. Start it up

```bash
docker compose up -d
```

Open [http://localhost:8080](http://localhost:8080), create an account, and you're in.

!!! tip "First account is yours"
    The first time you visit, you'll land on the sign-in screen. Click **Sign up** to register your account — there's no extra "admin user" step.

## What's next?

<div class="grid cards" markdown>

-   :material-key-variant:{ .lg .middle } **[Get your AeroDataBox API key](api-key.md)**

    ---

    The one external dependency. Free tier, two minutes.

-   :material-cog-outline:{ .lg .middle } **[Configuration reference](configuration.md)**

    ---

    Every environment variable Flightlog reads.

-   :material-cloud-upload-outline:{ .lg .middle } **[Deployment recipes](deployment.md)**

    ---

    Traefik labels, reverse-proxy notes, persistent volumes.

-   :material-airplane-takeoff:{ .lg .middle } **[Log your first flight](../usage/search-and-add.md)**

    ---

    The fun part.

</div>
