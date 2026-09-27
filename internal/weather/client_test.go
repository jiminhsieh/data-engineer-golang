package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientSuccessAndQuery(t *testing.T) {
	body := fixture(t, "weather_complete.json")
	var calls atomic.Int64
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Query().Get("appid") != "top-secret" || r.URL.Query().Get("lat") != "44.34" {
			t.Error("bad query")
		}
		_, _ = w.Write(body)
	}))
	defer s.Close()
	got, err := NewClient(s.URL, "top-secret", 44.34, 10.99, time.Second).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Raw) != string(body) {
		t.Fatal("raw bytes changed")
	}
	if calls.Load() != 1 {
		t.Fatal("not one request")
	}
}

func TestClientFailuresAreSingleAttemptAndSecretSafe(t *testing.T) {
	tests := []struct {
		name    string
		handler http.Handler
		max     int64
		timeout time.Duration
	}{
		{"status", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no", 500) }), 1024, time.Second},
		{"oversized", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(strings.Repeat("x", 20))) }), 10, time.Second},
		{"malformed", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{")) }), 1024, time.Second},
		{"identity", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"dt":1,"main":{"temp":1}}`)) }), 1024, time.Second},
		{"timeout", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(30 * time.Millisecond)
			_, _ = w.Write([]byte(`{}`))
		}), 1024, time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int64
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); tt.handler.ServeHTTP(w, r) }))
			defer s.Close()
			c := NewClientWithHTTP(s.URL, "top-secret", 1, 1, &http.Client{Timeout: tt.timeout}, tt.max)
			_, err := c.Fetch(context.Background())
			if err == nil {
				t.Fatal("wanted error")
			}
			if strings.Contains(err.Error(), "top-secret") {
				t.Fatal("secret leaked")
			}
			if calls.Load() != 1 {
				t.Fatalf("calls=%d", calls.Load())
			}
		})
	}
}
