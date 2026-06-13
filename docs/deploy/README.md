# Deploying Circl to production

This folder is the **generic deploy template**. It's infrastructure-agnostic
— the same recipe works on a single VPS, a self-hosted server, a managed
container host, or a Raspberry Pi. The compose file, env example, and nginx
sample are starting points: copy them somewhere outside the repo (a private
`deploy/` folder works; it's gitignored), customise for your environment,
and commit your operator-specific runbook in your own ops repo.

For the maintainer's own Pi + YunoHost runbook, see the local `deploy/`
folder on the machine running it (intentionally not tracked here).

## Architecture

Five containers, one network, one host-bound HTTP entry point you put behind
your own reverse proxy / TLS terminator.

| Service       | Image / source                                | Internal port | Exposed |
|---------------|-----------------------------------------------|---------------|---------|
| `postgres`    | `postgres:17-alpine`                          | 5432          | no      |
| `redis`       | `redis:7-alpine`                              | 6379          | no      |
| `moderation`  | `ghcr.io/mayloo89/circl-moderation` (or `../ops/moderation`) | 8000 | no |
| `backend`     | `ghcr.io/mayloo89/circl-backend` (or `../backend`)   | 8080  | 127.0.0.1:8080 |
| `frontend`    | `ghcr.io/mayloo89/circl-frontend` (or `../frontend`) | 3000  | 127.0.0.1:3000 |

The three app images are published to GHCR (**linux/arm64 only** — the
project's production target is a Raspberry Pi 5) by the CI `publish-images`
job on every green push to `develop` or `main`, tagged with the branch name
and the short commit SHA (`sha-<hash>`). The compose file defaults to the
`:develop` tag; override per service with `BACKEND_IMAGE` /
`FRONTEND_IMAGE` / `MODERATION_IMAGE` in `.env.prod` (that's also the
rollback lever — pin a `sha-<hash>` tag).

**On amd64 (no published images) it still works.** `update.sh` pulls with
`--ignore-pull-failures`, so any image that can't be fetched (wrong arch, or
the frontend image skipped because `DEPLOY_DOMAIN` is unset) falls through to
`compose up` building it from the service's `build:` clause — the first run
builds from source automatically rather than erroring. To pull in *new* code
on amd64, run `update.sh --source` (the plain run reuses the locally built
image). Multi-arch publishing would remove the build step entirely.

> **Frontend image is domain-specific.** `NEXT_PUBLIC_*` values (API URL,
> image host, Turnstile site key) bake into the browser bundle at build
> time. The published frontend image targets the domain in the repo's
> `DEPLOY_DOMAIN` GitHub Actions variable (plus the optional
> `NEXT_PUBLIC_TURNSTILE_SITE_KEY` variable); when `DEPLOY_DOMAIN` is unset
> the frontend image is skipped. Forks or alternate domains must build the
> frontend from source (`update.sh --source`) or publish their own image.

The reverse proxy on the host terminates TLS and routes:
- `/api/*` → `127.0.0.1:8080` (backend)
- `/*` → `127.0.0.1:3000` (frontend)
- `/ws` → `127.0.0.1:8080` (WebSocket upgrade)

Uploaded files live in a named Docker volume (`uploads_data`) and are served
from the backend's `/uploads/files/` path — no S3 / MinIO needed for a
single-host deploy. The `LOCAL_STORAGE_BASE_URL` env var tells the backend
the public URL the proxy serves them from.

## Files in this folder

- **`docker-compose.prod.yml`** — production compose. Pulls the published
  GHCR images by default; the `build:` contexts point at `../backend`,
  `../frontend`, `../ops/moderation` as a from-source fallback, so the file
  expects to be one level deep relative to the repo (e.g. `deploy/`).
- **`update.sh`** — pull-based updater: `compose pull && compose up -d`
  plus image pruning. `--source` rebuilds from the working tree instead.
- **`.env.prod.example`** — every env var the stack reads today. Copy it
  to your local `deploy/.env.prod` and fill in the `CHANGE_ME` values.
- **`nginx.example.conf`** — a generic reverse-proxy vhost. Replace the
  `EXAMPLE_DOMAIN` placeholder and the TLS cert paths to match your
  certificate manager (Let's Encrypt via certbot, Caddy, your control
  panel, etc.).

## Step-by-step

### 1. Prerequisites on the host

- Docker Engine 24+ with the `docker compose` plugin (`curl -fsSL https://get.docker.com | sh`).
- A reverse proxy that can terminate TLS and forward to `127.0.0.1`. Any
  of nginx, Caddy, Traefik, HAProxy works.
- DNS pointing your domain at the host's public IP.
- An SMTP provider for transactional email (Brevo / Mailgun / SendGrid /
  Amazon SES). Required for password reset, email verification, appeal /
  export notifications.

### 2. Clone and stage your local deploy folder

The `deploy/` directory is gitignored — your operator-specific files
(real `.env.prod`, your customised nginx vhost, your runbook) live there
and never make it back to the repo:

```bash
git clone https://github.com/mayloo89/circl.git /opt/circl
cd /opt/circl

mkdir -p deploy
cp docs/deploy/docker-compose.prod.yml  deploy/
cp docs/deploy/update.sh                deploy/
cp docs/deploy/.env.prod.example        deploy/.env.prod
cp docs/deploy/nginx.example.conf       deploy/nginx.conf
```

### 3. Fill in secrets

```bash
chmod 600 deploy/.env.prod
nano    deploy/.env.prod
```

Generators for the random secrets:

```bash
openssl rand -base64 32                   # POSTGRES_PASSWORD, JWT_SECRET, AUTH_SECRET (three different values)
npx web-push generate-vapid-keys          # VAPID_PUBLIC_KEY + VAPID_PRIVATE_KEY
```

Final sanity check:

```bash
grep CHANGE_ME deploy/.env.prod   # must print nothing
```

### 4. Install the reverse-proxy vhost

Replace the placeholder and drop the file wherever your proxy reads from
(`/etc/nginx/conf.d/`, `/etc/caddy/Caddyfile.d/`, etc.):

```bash
sed -i 's/EXAMPLE_DOMAIN/your.domain.com/g' deploy/nginx.conf
sudo cp deploy/nginx.conf /etc/nginx/conf.d/your.domain.com.conf
sudo nginx -t && sudo systemctl reload nginx
```

If your TLS cert lives somewhere other than `/etc/letsencrypt/live/<domain>/`,
edit the `ssl_certificate*` lines too.

### 5. Pull and start

The published images are pulled from GHCR — no compilation on the host
(the moderation image alone saves a ~2 GB PyTorch + NudeNet build that
takes 15–25 min on a single-board host):

```bash
cd /opt/circl
./deploy/update.sh
docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod.yml logs -f
```

Building from source instead (fork / custom domain):

```bash
docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod.yml up -d --build
```

Expected startup order:
1. `postgres` becomes healthy → `redis` becomes healthy.
2. `moderation` warms the NudeNet model (~60 s start period).
3. `backend` runs database migrations, starts on `:8080`.
4. `frontend` starts on `:3000`.

### 6. Verify

```bash
# Internal health endpoint
curl https://your.domain.com/api/health
# Expected: {"status":"ok","db":"ok","redis":"ok"}

# All containers should be healthy
docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod.yml ps
```

## Updating

```bash
/opt/circl/deploy/update.sh
```

That pulls the latest published images for the tag the compose file tracks
(`:develop` by default) and restarts only the services whose image changed.
Database migrations run automatically when the backend starts. Building
from source instead: `update.sh --source` (runs `git pull` + `compose
build`; remember plain `restart` is NOT enough to ship new code).

**Rollback**: every push publishes a `sha-<hash>` tag. Pin it in
`.env.prod` and re-run the updater:

```bash
echo 'BACKEND_IMAGE=ghcr.io/mayloo89/circl-backend:sha-abc1234' >> deploy/.env.prod
/opt/circl/deploy/update.sh
```

**Auto-deploy (optional)**: for unattended continuous deployment from
`develop`, run the updater on a schedule — e.g. a systemd timer or cron
entry on the host:

```cron
*/15 * * * * /opt/circl/deploy/update.sh >> /var/log/circl-update.log 2>&1
```

Pulls are cheap no-ops when the tag hasn't moved. Skip this if you prefer
deliberate, hands-on releases.

If a CDN sits in front of your reverse proxy (Cloudflare, Fastly, etc.) and
the visible result doesn't reflect the new code, purge the CDN cache — the
frontend bundle is fingerprinted but the HTML shell is sometimes held.

## Troubleshooting

### Frontend reports `(unhealthy)` even though the site responds

Two known quirks worth knowing about, both already handled by the compose
file in this folder. If you're customising it, don't undo them:

1. **`HOSTNAME: 0.0.0.0`** must be set on the frontend service. Next.js v16
   standalone binds to the value of `HOSTNAME` rather than defaulting to
   `0.0.0.0`. Without this it attaches to the container ID hostname and the
   Dockerfile healthcheck (`wget localhost:3000`) gets "Connection refused".

2. **Override the Dockerfile healthcheck** with `pgrep -f next-server`.
   BusyBox `wget` (used in the alpine-based frontend image) resolves
   `localhost` to IPv6 `::1` first, but Next.js standalone only binds to
   IPv4 `0.0.0.0`. So the loopback HTTP probe gets "Connection refused"
   even though external traffic via the docker bridge works (curl from the
   host to the container IP returns 307). Process-existence is the more
   reliable liveness signal here.

### Moderation sidecar advisory: kernel cgroup memory not enabled

Some Linux kernels (notably Raspberry Pi OS) don't enable the memory cgroup
controller by default. Docker logs a warning and runs the moderation
container without enforcing the 1 GB limit hint in the compose file.
NudeNet uses ~500 MB in steady state, so this is informational, not a real
problem on most hardware. Enable the controller in your kernel boot args if
you need enforcement.

### First moderation build is very slow / runs out of disk

The sidecar pulls PyTorch wheels and the NudeNet ONNX model at build time
(~2 GB downloaded, ~700 MB image). On small SSDs / SD cards, prune the
build cache first (`docker builder prune -af`) and consider mounting Docker
at an external volume (`/etc/docker/daemon.json` → `"data-root"`).

### Migrations failed on startup

```bash
docker logs <project>-backend-1 --tail 50
```

Most common cause: `POSTGRES_PASSWORD` in `.env.prod` doesn't match the
password in `DATABASE_URL`. Both must be identical, or set
`DATABASE_URL` once and let the compose interpolate.
