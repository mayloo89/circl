# Deploy on Raspberry Pi with YunoHost

This guide deploys Circl on a Raspberry Pi running YunoHost. Docker runs the app alongside YunoHost without touching YunoHost's own apps.

**Stack on the Pi:**
- YunoHost manages nginx (ports 80/443) and SSL certificates
- Docker Compose runs: backend (Go), frontend (Next.js), PostgreSQL, Redis
- nginx proxies `circl.unlug.ar` → Docker containers on localhost

**Prerequisites:**
- Raspberry Pi 4 or 5, 4 GB RAM minimum
- YunoHost already configured with a domain and working HTTPS
- Port 80 and 443 forwarded to the Pi on your router
- A subdomain available (e.g. `circl.unlug.ar`)

---

## Step 1 — Install Docker

YunoHost and Docker coexist without conflict. Install Docker via the official script:

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
# Log out and back in, or run: newgrp docker
```

Verify:
```bash
docker run --rm hello-world
```

---

## Step 2 — Add the subdomain in YunoHost

```bash
# Add the subdomain (replace with your actual subdomain)
sudo yunohost domain add circl.unlug.ar

# Install a Let's Encrypt SSL certificate for it
sudo yunohost domain cert install circl.unlug.ar
```

This creates `/etc/nginx/conf.d/circl.unlug.ar.conf` (YunoHost's default config).
You will replace it in Step 5.

---

## Step 3 — Clone the repo

```bash
cd /opt
sudo git clone https://github.com/mayloo89/circl.git
sudo chown -R $USER:$USER /opt/circl
cd /opt/circl
```

---

## Step 4 — Configure environment variables

```bash
cp deploy/.env.prod.example .env.prod
nano .env.prod
```

Fill in every `CHANGE_ME` value:

| Variable | How to get the value |
|----------|----------------------|
| `POSTGRES_PASSWORD` | `openssl rand -base64 32` |
| `JWT_SECRET` | `openssl rand -base64 32` |
| `AUTH_SECRET` | `openssl rand -base64 32` |
| `VAPID_PUBLIC_KEY` / `VAPID_PRIVATE_KEY` | `npx web-push generate-vapid-keys` |
| `SMTP_*` | Your email provider credentials (see note below) |

> **Email provider**: Circl requires SMTP to send verification emails and password resets.
> Free options: [Brevo](https://brevo.com) (300 emails/day) or [Resend](https://resend.com) (100/day).
> Create a free account, get SMTP credentials, and fill them in.

Also update `DOMAIN`, `FRONTEND_URL`, `CORS_ALLOWED_ORIGINS`, `LOCAL_STORAGE_BASE_URL`,
`SMTP_FROM`, and `VAPID_SUBJECT` with your actual domain.

---

## Step 5 — Install the nginx config

Replace YunoHost's default config for this domain with the Circl proxy config:

```bash
# Substitute CIRCL_DOMAIN with your actual subdomain in the config file
sed 's/CIRCL_DOMAIN/circl.unlug.ar/g' deploy/nginx/circl.conf \
  | sudo tee /etc/nginx/conf.d/circl.unlug.ar.conf

# Test and reload
sudo nginx -t
sudo systemctl reload nginx
```

> **Note:** When YunoHost renews your SSL certificate (automatic, every ~90 days),
> it does NOT overwrite this file — it only regenerates configs for its own installed apps.
> Your config is safe across cert renewals.

---

## Step 6 — Build and start

The first build compiles the Go binary and the Next.js bundle. On a Pi 5 this takes
roughly 10–15 minutes.

```bash
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --build
```

Watch the logs to confirm everything started cleanly:

```bash
docker compose -f docker-compose.prod.yml logs -f
```

Expected startup sequence:
1. `postgres` and `redis` become healthy
2. `backend` runs migrations and starts on port 8080
3. `frontend` starts on port 3000

---

## Step 7 — Verify

```bash
# Backend health check
curl https://circl.unlug.ar/api/health

# Should return something like:
# {"status":"ok","db":"ok","redis":"ok","version":"..."}
```

Then open `https://circl.unlug.ar` in a browser and register an account.

---

## Pointing circl.ar at the same server

Once the app is working on `circl.unlug.ar`, you can point your `circl.ar` domain at it:

1. In your DNS registrar, add an A record: `circl.ar → <your Pi's public IP>`
   (or a CNAME `circl.ar → unlug.ar` if your registrar supports APEX CNAME)
2. Add the domain to YunoHost and get a cert:
   ```bash
   sudo yunohost domain add circl.ar
   sudo yunohost domain cert install circl.ar
   ```
3. Copy the nginx config for the new domain:
   ```bash
   sed 's/CIRCL_DOMAIN/circl.ar/g' deploy/nginx/circl.conf \
     | sudo tee /etc/nginx/conf.d/circl.ar.conf
   sudo nginx -t && sudo systemctl reload nginx
   ```
4. Update `DOMAIN`, `FRONTEND_URL`, `CORS_ALLOWED_ORIGINS`, `LOCAL_STORAGE_BASE_URL`,
   and `AUTH_URL` in `.env.prod` to use `circl.ar`, then rebuild:
   ```bash
   docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --build frontend
   ```

---

## Updating the app

```bash
cd /opt/circl
git pull origin main
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --build
```

Database migrations run automatically on backend startup.

---

## Useful commands

```bash
# View live logs
docker compose -f docker-compose.prod.yml logs -f

# View logs for one service
docker compose -f docker-compose.prod.yml logs -f backend

# Restart one service
docker compose -f docker-compose.prod.yml restart backend

# Stop everything
docker compose -f docker-compose.prod.yml down

# Stop and delete all data (destructive!)
docker compose -f docker-compose.prod.yml down -v
```
