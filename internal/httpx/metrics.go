package httpx

import (
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	reqCounter       metric.Int64Counter
	errCounter       metric.Int64Counter
	latencyHistogram metric.Float64Histogram
)

func init() {
	m := Meter()
	reqCounter, _ = m.Int64Counter("http.server.requests",
		metric.WithDescription("Total number of HTTP requests"))
	errCounter, _ = m.Int64Counter("http.server.errors",
		metric.WithDescription("Total number of HTTP error responses (status >= 400)"))
	latencyHistogram, _ = m.Float64Histogram("http.server.duration.ms",
		metric.WithDescription("Request duration in milliseconds"))
}

type statusCapturingWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusCapturingWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusCapturingWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sw, r)

		attrs := []attribute.KeyValue{
			attribute.String("http.method", r.Method),
			attribute.String("http.route", r.URL.Path),
			attribute.Int("http.status_code", sw.status),
		}

		reqCounter.Add(r.Context(), 1, metric.WithAttributes(attrs...))
		latencyHistogram.Record(r.Context(), float64(time.Since(start).Milliseconds()), metric.WithAttributes(attrs...))
		if sw.status >= 400 {
			errCounter.Add(r.Context(), 1, metric.WithAttributes(attrs...))
		}
	})
}
