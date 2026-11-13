# Release notes

What changed in each release, and anything you need to know before upgrading. Releases that only update dependencies are not listed here; the full list is on the [GitHub releases page](https://github.com/thulasirajkomminar/flightlog/releases).

## v0.13.1

### Fixed

- **The container failed to start with `chown: /app/data: Operation not permitted`** when the data directory lives on storage that doesn't allow changing ownership (NFS or SMB shares, rootless Docker or Podman, or a container with dropped capabilities). It now logs a short notice and keeps running as root there, as versions before v0.13.0 did. See [where data lives](getting-started/configuration.md#where-data-lives).

If you are on v0.13.0 and hit this, upgrade to v0.13.1. Until then, adding `entrypoint: ["./main"]` to the service in your compose file starts the app the old way.

## v0.13.0

A large release: far fewer AeroDataBox API calls, a globe map, background imports, flights added by hand, and account settings.

### Before you upgrade

!!! warning "Back up your data directory first"
    The database migrates itself on first start, and some of the changes cannot be undone. Copy the volume or bind-mount that holds `flightlog.db` before pulling the new image.

No manual migration steps are needed. To upgrade, follow the usual [upgrade steps](getting-started/deployment.md#upgrading).

### What changes on first start

- **Unused data is removed from the database:** the route distance cache, the stored raw AeroDataBox responses, and four redundant distance columns. Nothing you can see in the app is lost.
- **Imported flights become private.** A flight you imported from a CSV now belongs to your account only. A flight that several accounts already share stays shared.
- **The container runs as an unprivileged user.** Existing volumes are made writable for it automatically. Where the storage doesn't allow that (NFS or SMB shares, rootless Docker, dropped capabilities), it keeps running as root and says so in the log.

### What behaves differently

- **Dashboard totals count flown flights only.** Cancelled and diverted flights no longer add to the map or the tiles. A flight still marked "scheduled" a day after its date counts as flown. See [Dashboard](usage/dashboard.md).
- **The dashboard map is a globe,** with a button for the flat view. It needs a browser with WebGL.
- **Flightlog makes a few API calls on its own:** one after a logged flight's scheduled arrival, to record that it landed, and at most three per flight. Set `FLIGHT_STATUS_REFRESH=false` to turn this off. See [how many calls Flightlog makes](getting-started/api-key.md#how-many-calls-will-i-make).
- **Sign-in is limited to 10 attempts per minute per address.** Behind a reverse proxy, set `SERVER_TRUSTED_PROXIES`, or everyone shares that limit.
- **Imports run in the background** and accept up to 2,000 flights instead of 100.
- **Changing your password signs out your other sessions,** and deleting an account ends its sessions at once.

### API changes

These only matter if you call the API from your own scripts.

- `POST /api/v1/import` now returns `202` with a job. Poll `GET /api/v1/import/status` for progress and the result.
- `GET /api/v1/providers/{provider}/flights/search` is removed. Use `GET /api/v1/flights/search`.

### After upgrading

- Set `AUTH_REGISTRATION_ENABLED=false` once your accounts exist. See [where to run it](getting-started/deployment.md#where-to-run-it).
- Check the log for a warning about a short `AUTH_JWT_SECRET`, and replace it if you see one.
- To run the binary by hand, use `docker compose exec -u flightlog flightlog ./main ...`, so files stay owned by the right user.

### New

- **Flights by hand:** add a flight the search doesn't find, and edit flights you added or imported. See [Search & add](usage/search-and-add.md#adding-a-flight-by-hand).
- **My Flights:** search across all years and filter by status.
- **Dashboard:** a closer look at your flights, with a calendar of when you flew, your top routes and airlines, and a few fun facts.
- **Imports:** a progress bar, a cancel button, and imports that carry on after a server restart.
- **Accounts:** change your password or delete your account under Settings, and reset a forgotten password with `./main reset-password <email>`. See [Accounts](usage/accounts.md).
- **API quota:** the remaining AeroDataBox quota is shown under Settings.
- **Built-in airport database:** about 9,000 airports, so imports and hand-added flights need no airport lookups.

### Fixed

- The dashboard map showed no tiles in production.
- The map was limited to 200 flights.
- Flights added before departure stayed "scheduled" forever.
- An import could link a row to another airline's flight on the same route and day.
- A failed lookup was always reported as "no flights found", even when AeroDataBox was unreachable or out of quota.
- Signing in failed over plain HTTP on a LAN address.
- Email addresses differing only in upper and lower case were treated as different accounts.
- Flight rows overflowed sideways on phone screens.
