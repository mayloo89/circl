# WebSocket load test

Validates that the chat hub holds up under many concurrent WebSocket
connections at expected peak — connection success rate, handshake latency,
and broadcast fan-out — using [k6](https://k6.io).

`ws-load-test.js` drives the real guest → public-room path: each virtual user
creates a guest session, redeems a single-use WS ticket, opens a WebSocket,
and sends a message every ~15s while receiving the room's broadcasts. **One VU
= one held-open connection**, so peak VUs = peak concurrent connections.

## Prerequisites

1. **k6** — `brew install k6` (macOS) or `docker run --rm -i grafana/k6` (see below).
2. **A public room exists.** The test discovers one via `GET /guest/rooms`, or
   pass `ROOM_ID=<id>`. Create one from the admin UI if there are none.
3. **The target backend must have per-IP rate limits disabled** — all load
   originates from a single IP, so the global limiter and guest limiter would
   otherwise reject most requests. Start the backend (or a dedicated load-test
   environment — **never production**) with:

   ```bash
   GLOBAL_IP_LIMIT=0 GUEST_IP_RATE=0 WS_IP_CONN_LIMIT=0 ...the rest... ./api
   ```

4. **Origin** — the WS upgrade validates `Origin` against
   `CORS_ALLOWED_ORIGINS`. The script sends `http://localhost:3000` by default;
   override with `ORIGIN=` to match the target's allowed origins.
5. **Turnstile must be off.** When `TURNSTILE_SECRET` is set, `POST
   /guest/session` requires a valid `captcha_token`, which the script doesn't
   provide — every session would 403 and look like an auth failure. Run the
   load-test backend with `TURNSTILE_SECRET` unset (captcha skipped).

## Run

```bash
# Local k6 against a local backend
k6 run loadtest/ws-load-test.js

# Crank the peak and point at a remote host
BASE_URL=https://circl.example.com ORIGIN=https://circl.example.com \
  PEAK_VUS=500 HOLD=5m k6 run loadtest/ws-load-test.js

# Via Docker (host backend reachable at host.docker.internal)
docker run --rm -i -e BASE_URL=http://host.docker.internal:8080 \
  grafana/k6 run - < loadtest/ws-load-test.js
```

## Configuration (env vars)

| Var | Default | Meaning |
|-----|---------|---------|
| `BASE_URL` | `http://localhost:8080` | Backend HTTP base URL. |
| `WS_URL` | derived from `BASE_URL` | WebSocket base (http→ws) — override for split hosts. |
| `ORIGIN` | `http://localhost:3000` | `Origin` header; must be in `CORS_ALLOWED_ORIGINS`. |
| `ROOM_ID` | discovered | Target public room; auto-picked from `GET /guest/rooms` if empty. |
| `PEAK_VUS` | `200` | Peak concurrent connections. |
| `RAMP` | `30s` | Ramp-up and ramp-down duration. |
| `HOLD` | `2m` | Time held at peak. |
| `CONN_LIFETIME_S` | `60` | How long each connection stays open. |
| `SEND_INTERVAL_S` | `15` | Send gap per connection (keep ≥ the guest message-rate window). |

## What it measures

Built-in k6 WebSocket metrics plus custom ones, gated by thresholds:

- `ws_connecting` (handshake latency) — **p95 < 1.5s**.
- `ws_connect_errors` — sockets that never opened — **< 5%**.
- `ws_auth_errors` — guest session / ticket HTTP failures — **< 2%**.
- `ws_app_msgs_received` — broadcast frames received (fan-out sanity).
- `ws_time_to_open_ms` — time from VU start to socket open.
- `checks` — overall pass rate — **> 95%**.

k6 exits non-zero if any threshold is breached, so this is CI/alert-friendly.

## Watching it live

If the observability stack is running (see `docs/deploy/README.md`), watch in
Grafana during the run:

- `circl_websocket_active_connections` should track the VU ramp.
- HTTP RED dashboard — `/guest/session` and `/guest/ws-ticket` latency/errors.
- `circl_panics_total` should stay flat — any increase is a hub/pump bug under
  load (the `RecoveredPanics` alert will fire).

## Interpreting results

- Rising `ws_connecting` p95 or climbing `ws_connect_errors` as VUs increase →
  you've found the connection ceiling for the host; note it and tune
  (`WS_IP_CONN_LIMIT` in real deployments, file-descriptor limits, CPU).
- `ws_app_msgs_received` near zero while connections succeed → fan-out/publish
  problem, not a connection problem.
- Climbing `circl_db_pool` saturation → the auth path (session creation) is the
  bottleneck, not the sockets.

This is a manual, on-demand tool — it is intentionally **not** wired into CI
(it needs a running backend with limits disabled and a public room).
