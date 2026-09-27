package observability

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Handler(metrics *Metrics, pinger Pinger) http.Handler {
	mux := http.NewServeMux()
	metricsHandler := promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{})
	mux.Handle("/metrics", getOnly(metricsHandler))
	mux.Handle("/healthz", getOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := "ok"
		code := http.StatusOK
		if pinger.Ping(r.Context()) != nil {
			status = "unhealthy"
			code = http.StatusServiceUnavailable
		}
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
	})))
	return mux
}
func getOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type PingFunc func(context.Context) error

func (f PingFunc) Ping(ctx context.Context) error { return f(ctx) }
