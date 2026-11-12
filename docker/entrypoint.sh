#!/bin/sh
# Runs Flightlog as the unprivileged "flightlog" user.
#
# The container starts as root only to make the data directory writable for
# that user: volumes created by earlier versions, and fresh bind mounts, are
# owned by root. If the container is already started as another user (for
# example with `user:` in compose), nothing is changed.
set -e

if [ "$(id -u)" = "0" ]; then
  chown -R flightlog:flightlog /app/data
  exec su-exec flightlog "$@"
fi

exec "$@"
