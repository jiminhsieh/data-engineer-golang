package postgres

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type fakeDB struct {
	calls int
	err   error
}

func (f *fakeDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	f.calls++
	return pgconn.NewCommandTag("INSERT 0 0"), f.err
}
func (f *fakeDB) Ping(context.Context) error { return nil }
func TestInsertAttemptsOnceOnError(t *testing.T) {
	f := &fakeDB{err: errors.New("down")}
	_, err := NewRepository(f).Insert(context.Background(), 1, 2, 3, []byte(`{}`))
	if err == nil || f.calls != 1 {
		t.Fatalf("err=%v calls=%d", err, f.calls)
	}
}

func TestRepositoryIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	_, _ = pool.Exec(ctx, "DROP TABLE IF EXISTS weather_raw")
	r := NewRepository(pool)
	if err := r.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"coord":{"lat":44.34,"lon":10.99},"dt":1767225600,"unknown":{"x":1}}`)
	created, err := r.Insert(ctx, 1767225600, 44.34, 10.99, raw)
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	created, err = r.Insert(ctx, 1767225600, 44.34, 10.99, raw)
	if err != nil || created {
		t.Fatalf("duplicate created=%v err=%v", created, err)
	}
	var count int
	var payload string
	var utc bool
	err = pool.QueryRow(ctx, `SELECT count(*), max(payload::text), bool_and(EXTRACT(timezone FROM ingested_at)=0) FROM weather_raw`).Scan(&count, &payload, &utc)
	if err != nil || count != 1 || !strings.Contains(payload, `"unknown": {"x": 1}`) || !utc {
		t.Fatalf("count=%d payload=%s utc=%v err=%v", count, payload, utc, err)
	}
}
