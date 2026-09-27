package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jiminhsieh/data-engineer-golang/internal/config"
)

type fakeDatabase struct {
	execs  atomic.Int64
	closed atomic.Bool
}

func (f *fakeDatabase) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	f.execs.Add(1)
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}
func (f *fakeDatabase) Ping(context.Context) error { return nil }
func (f *fakeDatabase) Close()                     { f.closed.Store(true) }
func appConfig(t *testing.T, baseURL string) config.Config {
	t.Helper()
	return config.Config{APIKey: "key", Latitude: 44.34, Longitude: 10.99, DatabaseURL: "postgres://user:secret@localhost/weather", APIBaseURL: baseURL, Interval: time.Hour, HTTPTimeout: time.Second, HTTPAddr: "127.0.0.1:0", DataDir: filepath.Join(t.TempDir(), "data"), LogFile: filepath.Join(t.TempDir(), "logs", "etl.log"), ShutdownGrace: time.Second}
}
func TestStartupDependencyFailureIsSecretSafe(t *testing.T) {
	cfg := appConfig(t, "https://example.com")
	err := run(context.Background(), cfg, func(context.Context, string) (database, error) { return nil, errors.New("database unavailable") })
	if err == nil || err.Error() != "database unavailable" {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestRunningServiceCancelsAndClosesResources(t *testing.T) {
	requested := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requested <- struct{}{}
		_, _ = w.Write([]byte(`{"coord":{"lon":10.99,"lat":44.34},"main":{"temp":20},"dt":1767225600,"cod":200}`))
	}))
	defer server.Close()
	cfg := appConfig(t, server.URL)
	db := &fakeDatabase{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, cfg, func(context.Context, string) (database, error) { return db, nil }) }()
	select {
	case <-requested:
	case <-time.After(time.Second):
		t.Fatal("pipeline did not start immediately")
	}
	deadline := time.Now().Add(time.Second)
	for db.execs.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("service did not stop")
	}
	if !db.closed.Load() {
		t.Fatal("database was not closed")
	}
}
