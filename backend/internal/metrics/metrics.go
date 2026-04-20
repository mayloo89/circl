package metrics

import (
	"net/http"
	"strconv"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// DBStatter is satisfied by *pgxpool.Pool.
type DBStatter interface {
	Stat() *pgxpool.Stat
}

// WSCounter is satisfied by any hub that tracks active WebSocket connections.
type WSCounter interface {
	ActiveConns() int64
}

// Metrics holds the Prometheus registry and custom instruments.
type Metrics struct {
	reg         *prometheus.Registry
	reqsTotal   *prometheus.CounterVec
	reqDuration *prometheus.HistogramVec
}

// New creates a Metrics instance with Go runtime, process, HTTP, and DB pool
// collectors registered. Call RegisterWSHub to add WebSocket connection tracking.
func New(pool DBStatter) *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	reqsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "circl_http_requests_total",
		Help: "Total HTTP requests partitioned by method, route pattern, and status code.",
	}, []string{"method", "path", "status"})

	reqDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "circl_http_request_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	reg.MustRegister(reqsTotal, reqDuration)

	if pool != nil {
		reg.MustRegister(&dbPoolCollector{pool: pool})
	}

	return &Metrics{reg: reg, reqsTotal: reqsTotal, reqDuration: reqDuration}
}

// RegisterWSHub adds a WebSocket active-connections gauge backed by hub.
func (m *Metrics) RegisterWSHub(hub WSCounter) {
	m.reg.MustRegister(&wsConnsCollector{
		desc: prometheus.NewDesc(
			"circl_websocket_active_connections",
			"Number of active WebSocket connections.",
			nil, nil,
		),
		hub: hub,
	})
}

// Handler returns an HTTP handler for the /metrics endpoint.
// If token is non-empty, requests must carry Authorization: Bearer <token>.
func (m *Metrics) Handler(token string) http.Handler {
	h := promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{Registry: m.reg})
	if token == "" {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// Middleware returns a chi middleware that records HTTP request counts and latency.
// It uses the chi route pattern (e.g. /profiles/{userID}) rather than the raw
// path to avoid cardinality explosion from user-supplied IDs.
func (m *Metrics) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			// After next.ServeHTTP returns chi has resolved the route pattern.
			path := r.URL.Path
			if rctx := chi.RouteContext(r.Context()); rctx != nil && rctx.RoutePattern() != "" {
				path = rctx.RoutePattern()
			}

			status := strconv.Itoa(ww.Status())
			m.reqsTotal.WithLabelValues(r.Method, path, status).Inc()
			m.reqDuration.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
		})
	}
}

// --- DB pool collector ---

type dbPoolCollector struct{ pool DBStatter }

var (
	dbAcquiredDesc = prometheus.NewDesc("circl_db_pool_acquired_conns", "Acquired connections in the pool.", nil, nil)
	dbIdleDesc     = prometheus.NewDesc("circl_db_pool_idle_conns", "Idle connections in the pool.", nil, nil)
	dbTotalDesc    = prometheus.NewDesc("circl_db_pool_total_conns", "Total connections open in the pool.", nil, nil)
	dbMaxDesc      = prometheus.NewDesc("circl_db_pool_max_conns", "Maximum connections the pool is configured to hold.", nil, nil)
)

func (c *dbPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- dbAcquiredDesc
	ch <- dbIdleDesc
	ch <- dbTotalDesc
	ch <- dbMaxDesc
}

func (c *dbPoolCollector) Collect(ch chan<- prometheus.Metric) {
	s := c.pool.Stat()
	ch <- prometheus.MustNewConstMetric(dbAcquiredDesc, prometheus.GaugeValue, float64(s.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(dbIdleDesc, prometheus.GaugeValue, float64(s.IdleConns()))
	ch <- prometheus.MustNewConstMetric(dbTotalDesc, prometheus.GaugeValue, float64(s.TotalConns()))
	ch <- prometheus.MustNewConstMetric(dbMaxDesc, prometheus.GaugeValue, float64(s.MaxConns()))
}

// --- WS connections collector ---

type wsConnsCollector struct {
	desc *prometheus.Desc
	hub  WSCounter
}

func (c *wsConnsCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.desc }

func (c *wsConnsCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, float64(c.hub.ActiveConns()))
}
