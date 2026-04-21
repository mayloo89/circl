# LogQL query examples

Loki runs on `http://localhost:3100`. Until Grafana is added (PR #59), query
via the Loki HTTP API or `logcli`:

```bash
logcli query '{container_name=~".*backend.*"}' --tail
```

All examples below target the Loki API; paste them into the Grafana Explore panel
once PR #59 is in place.

---

## Backend (zerolog JSON)

**All backend log lines**
```logql
{container_name=~".*backend.*"}
```

**Errors only** (indexed label — no per-line scan)
```logql
{container_name=~".*backend.*", level="error"}
```

**Specific component**
```logql
{container_name=~".*backend.*", component="chat"}
{container_name=~".*backend.*", component="image_worker"}
{container_name=~".*backend.*", component="push_worker"}
```

**HTTP access log for a route prefix**
```logql
{container_name=~".*backend.*"} | json | path =~ `/api/chat.*`
```

**Slow requests (latency_ms > 500)**
```logql
{container_name=~".*backend.*"} | json | latency_ms > 500
```

**Log-trace correlation** — jump from a Tempo trace to its logs:
```logql
{container_name=~".*backend.*"} | json | trace_id="<paste-trace-id>"
```

**Error rate (5-min window)**
```logql
rate({container_name=~".*backend.*", level="error"}[5m])
```

**Top error messages**
```logql
topk(10,
  sum by (message) (
    count_over_time({container_name=~".*backend.*", level="error"} | json [10m])
  )
)
```

---

## Infrastructure

**Postgres errors**
```logql
{container_name=~".*postgres.*"} |= "ERROR"
```

**Redis**
```logql
{container_name=~".*redis.*"}
```

**Asynq worker failures**
```logql
{container_name=~".*backend.*", component="image_worker"} | json | level="error"
```

---

## Label reference

| Label          | Source            | Values                                       |
|----------------|-------------------|----------------------------------------------|
| `container_name` | docker discovery | `circl-backend-1`, `circl-postgres-1`, …    |
| `env`          | static (Alloy)    | `dev`                                        |
| `level`        | zerolog JSON      | `trace` `debug` `info` `warn` `error` `fatal` |
| `component`    | zerolog JSON      | `chat`, `image_worker`, `push_worker`, …     |
