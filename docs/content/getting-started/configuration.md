# Configuration

Everything Flightlog needs to know lives in environment variables — usually pulled from an `.env` file next to your `docker-compose.yml`.

The list is intentionally short. There are two required variables; everything else has a sensible default.

## Required

| Variable              | What it is                                                                                                                                                                                  |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `AERODATABOX_API_KEY` | Your RapidAPI key for AeroDataBox. [How to get one](api-key.md).                                                                                                                            |
| `AUTH_JWT_SECRET`     | The secret used to sign your login session token. **Use a strong, random value** — anything that can predict this can mint a session for any user. Generate with `openssl rand -base64 32`. |

If either of these is missing, Flightlog will refuse to start.

## Optional

| Variable      | Default      | What it does                                                                                                                                                   |
| ------------- | ------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `ENVIRONMENT` | `production` | Set to `development` to enable verbose console output and debug-mode UI affordances. Leave at `production` for any real deployment.                            |
| `SERVER_PORT` | `8080`       | The port Flightlog listens on inside the container. You usually don't need to change this — change the host-side port mapping in `docker-compose.yml` instead. |
| `LOG_LEVEL`   | `info`       | One of `debug`, `info`, `warn`, `error`. Bump to `debug` when something's not behaving and you want to see what's happening.                                   |

## Example `.env`

```bash title=".env"
# --- Required ---
AERODATABOX_API_KEY=abc123_your_rapidapi_key
AUTH_JWT_SECRET=PLEASE_GENERATE_THIS_WITH_openssl_rand_base64_32

# --- Optional ---
ENVIRONMENT=production
SERVER_PORT=8080
LOG_LEVEL=info
```

!!! warning "Rotate the JWT secret carefully"
    Changing `AUTH_JWT_SECRET` immediately invalidates every existing login session — you (and any other user on the instance) will be logged out and have to sign back in. That's a feature, not a bug: it's how you boot a leaked secret. Just don't be surprised when it happens.

## Where data lives

Flightlog stores everything — users, flights, cached lookups — in a single SQLite database at `/app/data/flightlog.db` inside the container.

In the recommended `docker-compose.yml`, that path is mounted from a named Docker volume:

```yaml
volumes:
  - flightlog_data:/app/data
```

To back up Flightlog, **back up that volume**. To migrate to a new host, copy the volume contents (or use [Export](../usage/import-export.md#export) and re-import on the new instance — both work).
