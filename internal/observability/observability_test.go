package observability

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogAppends(t *testing.T) {
	p := filepath.Join(t.TempDir(), "logs", "etl.log")
	l, e := OpenLog(p, nil)
	if e != nil {
		t.Fatal(e)
	}
	l.Print("first")
	_ = l.Close()
	l, e = OpenLog(p, nil)
	if e != nil {
		t.Fatal(e)
	}
	l.Print("second api_key=[REDACTED]")
	_ = l.Close()
	b, _ := os.ReadFile(p)
	s := string(b)
	if !strings.Contains(s, "first") || !strings.Contains(s, "second") || strings.Index(s, "first") > strings.Index(s, "second") {
		t.Fatal(s)
	}
}
func TestMetricsAndHandlers(t *testing.T) {
	m := NewMetrics()
	m.APITotal.Inc()
	m.APISuccess.Inc()
	m.TransformTotal.Inc()
	m.TransformErrors.Inc()
	m.DataSaved.WithLabelValues("raw").Inc()
	h := Handler(m, PingFunc(func(context.Context) error { return nil }))
	r := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	body := w.Body.String()
	for _, name := range []string{"weather_api_requests_total", "weather_api_requests_success_total", "weather_api_requests_failure_total", "etl_transform_total", "etl_transform_success_total", "etl_transform_errors_total", "etl_data_saved_total{destination=\"raw\"}"} {
		if !strings.Contains(body, name) {
			t.Errorf("missing %s", name)
		}
	}
	if strings.Contains(body, "go_gc") || strings.Contains(body, "secret") {
		t.Fatal("unexpected metric")
	}
	for _, tc := range []struct {
		err    error
		code   int
		status string
	}{{nil, 200, "ok"}, {errors.New("down"), 503, "unhealthy"}} {
		hh := Handler(NewMetrics(), PingFunc(func(context.Context) error { return tc.err }))
		ww := httptest.NewRecorder()
		hh.ServeHTTP(ww, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		b, _ := io.ReadAll(ww.Result().Body)
		if ww.Code != tc.code || !strings.Contains(string(b), tc.status) || strings.Contains(string(b), "down") {
			t.Fatalf("%d %s", ww.Code, b)
		}
	}
	for _, path := range []string{"/healthz", "/metrics"} {
		ww := httptest.NewRecorder()
		h.ServeHTTP(ww, httptest.NewRequest(http.MethodPost, path, nil))
		if ww.Code != 405 {
			t.Fatal(path, ww.Code)
		}
	}
}
