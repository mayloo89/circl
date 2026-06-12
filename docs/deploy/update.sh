#!/usr/bin/env bash
# Pull-based update: fetch the latest published images and restart whatever
# changed. Run from the host, any directory. Database migrations run
# automatically when the new backend starts.
#
# Roll back by pinning a previous build in .env.prod first, e.g.
#   BACKEND_IMAGE=ghcr.io/mayloo89/circl-backend:sha-<hash>
#
# Pass --source to rebuild from the working tree instead of pulling
# (fork / custom-domain workflow): the script then runs `git pull` +
# `compose build`.
set -euo pipefail
cd "$(dirname "$0")"

compose() {
  docker compose --env-file .env.prod -f docker-compose.prod.yml "$@"
}

if [[ "${1:-}" == "--source" ]]; then
  git -C .. pull --ff-only
  compose build
else
  compose pull
fi

compose up -d
docker image prune -f
compose ps
