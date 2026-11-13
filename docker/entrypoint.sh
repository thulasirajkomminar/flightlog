#!/bin/sh
# Runs Flightlog as the unprivileged "flightlog" user when the environment
# allows it, and otherwise as the user the container was started with.
#
# The container starts as root only to make the data directory writable for
# that user: volumes created by earlier versions, and fresh bind mounts, are
# owned by root. Some setups do not allow that hand-over (NFS or SMB mounts,
# rootless Docker or Podman, a dropped CHOWN or SETUID capability). There the
# app keeps running as root, as it did before, rather than refusing to start.
set -e

DATA_DIR=/app/data
DB_FILE="$DATA_DIR/flightlog.db"

# Already started as another user (for example with `user:` in compose).
if [ "$(id -u)" != "0" ]; then
  exec "$@"
fi

# Best effort: a failure here is handled by the check below.
chown -R flightlog:flightlog "$DATA_DIR" 2>/dev/null || true

# Drop privileges only if the unprivileged user can really use the data.
if su-exec flightlog sh -c "test -w '$DATA_DIR' && { [ ! -e '$DB_FILE' ] || test -w '$DB_FILE'; }" 2>/dev/null; then
  exec su-exec flightlog "$@"
fi

echo "flightlog: cannot run as the unprivileged user here (could not take ownership of $DATA_DIR or switch user), so the app runs as root." >&2
echo "flightlog: this is safe to ignore. To run unprivileged, make the data directory owned by UID 10001 on the host and start the container with user 10001:10001." >&2
exec "$@"
