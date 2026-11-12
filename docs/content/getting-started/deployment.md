# Deployment

The quickstart gets you a working Flightlog on `localhost:8080`. This page covers the next step: putting it on a real domain, with TLS, in a way that survives reboots.

The two recipes below are the ones I run myself. Pick the one that matches your existing setup.

## Where to run it

Flightlog is built for a **homelab**: one person or a household, on hardware you control. It is not hardened to be an open service on the public internet.

!!! warning "Keep it on a private network"
    The recommended setup is to run Flightlog on your home network and reach it from outside through a private overlay network such as [Tailscale](https://tailscale.com), [WireGuard](https://www.wireguard.com) or a similar VPN — not through a port opened to the internet.

That matters for two reasons. Anyone who can reach the sign-in page can create an account unless you close sign-ups, and every account's searches spend **your** AeroDataBox quota. And while the app has the usual protections (hashed passwords, rate limits, security headers), it has no email verification, no admin tools and no monitoring — things a public service needs.

Whichever recipe you pick, do these three things once it is running:

1. **Create your account(s), then close sign-ups** by setting `AUTH_REGISTRATION_ENABLED=false` (see [Configuration](configuration.md#optional)).
2. **Use a strong `AUTH_JWT_SECRET`** — `openssl rand -base64 32`. Flightlog warns in its log if it is too short.
3. **Back up the data directory.** Everything lives in one SQLite file; see the [persistence checklist](#persistence-checklist) below.

If you do put it behind a public reverse proxy anyway (Recipe 2), also set `SERVER_TRUSTED_PROXIES` to your proxy's network. Without it every visitor looks like the proxy, so the sign-in rate limit is shared by everyone.

## Recipe 1 — Plain Docker, single port

The simplest possible deployment: Flightlog listening on a port, you handle TLS some other way (or you don't need it because it's only on your LAN).

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

```bash
docker compose up -d
```

That's it. Browse to `http://<host>:8080`.

!!! tip "LAN-only is a perfectly valid threat model"
    Not every self-hosted app needs to be public. If Flightlog only ever needs to be reachable from devices on your home network, skip TLS entirely — bind it to a private port and call it done.

## Recipe 2 — Behind Traefik with automatic TLS

If you're already running [Traefik](https://traefik.io/) as your reverse proxy, drop Flightlog into your existing stack with these labels. The example assumes you have a `cloudflare` cert resolver and a shared `proxy` network — adjust the names to match your setup.

```yaml title="docker-compose.yml"
services:
  flightlog:
    container_name: flightlog
    image: ghcr.io/thulasirajkomminar/flightlog:latest
    environment:
      - AERODATABOX_API_KEY=${AERODATABOX_API_KEY}
      - AUTH_JWT_SECRET=${AUTH_JWT_SECRET}
      - SERVER_TRUSTED_PROXIES=172.18.0.0/16
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

A few things worth calling out in this version:

- **No published port.** Traffic reaches Flightlog over the `proxy` Docker network — there's nothing on the host's port 8080.
- **Bind-mount instead of volume.** `/opt/docker/flightlog/data` on the host is mapped to `/app/data` in the container. Easier to back up with the rest of your `/opt/docker` tree.
- **`environment:` instead of `env_file:`.** Lets you keep one `.env` at the top of your stack for all services.

## Persistence checklist

Wherever you deploy, before you start adding flights:

- [x] The SQLite database lives somewhere persistent (named volume **or** bind-mount).
- [x] That location is included in whatever backup mechanism you trust.
- [x] `AUTH_JWT_SECRET` is stored somewhere you can recover (a password manager works) — losing it means losing the ability to keep signed-in sessions stable.

## Upgrading

Flightlog publishes a `latest` tag and a `:vX.Y.Z` tag per release. To upgrade:

```bash
docker compose pull
docker compose up -d
```

The database migrates itself on startup — there's no separate migration step to run.

!!! note "Pin to a version if you prefer"
    For production-ish setups, replace `:latest` with the specific version tag you've tested. You'll see the available tags on the [GitHub Container Registry page](https://github.com/thulasirajkomminar/flightlog/pkgs/container/flightlog).
