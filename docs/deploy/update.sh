#!/usr/bin/env bash
# Pull-based update: fetch the latest published images and restart whatever
# changed. Run from the host, any directory. Database migrations run
# automatically when the new backend starts.
#
# Roll back by pinning a previous build in .env.prod first, e.g.
#   BACKEND_IMAGE=ghcr.io/mayloo89/circl-backend:sha-<hash>
#
# Pass --source to rebuild from the working tree instead of pulling
# (the way to ship updates on amd64, where no images are published): the
# script then runs `git pull` + `compose build`.
set -euo pipefail
cd "$(dirname "$0")"

compose() {
  docker compose --env-file .env.prod -f docker-compose.prod.yml "$@"
}

if [[ "${1:-}" == "--source" ]]; then
  git -C .. pull --ff-only
  compose build
else
  # Published images are linux/arm64 only, and the frontend image is skipped
  # when DEPLOY_DOMAIN is unset in CI. --ignore-pull-failures keeps the pull
  # non-fatal so any image that can't be fetched (wrong arch, not published)
  # falls through to `compose up` building it from the service's build:
  # clause, instead of aborting the whole update.
  compose pull --ignore-pull-failures
fi

# Without --build: services whose image was pulled run as-is; any still
# missing locally are built from their build: clause. So arm64 hosts never
# build, and amd64 / unpublished-image hosts build only what they must.
compose up -d
docker image prune -f
compose ps
