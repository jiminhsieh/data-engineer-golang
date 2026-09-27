package lake

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/jiminhsieh/data-engineer-golang/internal/transform"
	"github.com/parquet-go/parquet-go"
)

type badEncoder struct{ calls int }

func (*badEncoder) Extension() string                { return ".json" }
func (b *badEncoder) Encode(io.Writer, []byte) error { b.calls++; return errors.New("broken") }
func ident() Identity                                { return Identity{SourceDT: 1767225600, Latitude: 44.34, Longitude: 10.99} }
func TestPath(t *testing.T) {
	got := Path("", "raw", ".json", ident())
	want := filepath.Join("raw", "dt=20260101", "1767225600_44.34_10.99.json")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
func TestRawExactAndDuplicate(t *testing.T) {
	root := t.TempDir()
	w := NewWriter(root, JSONEncoder{})
	raw := []byte("{ \"exact\": true }\n")
	created, e := w.WriteRaw(ident(), raw)
	if e != nil || !created {
		t.Fatal(created, e)
	}
	path := Path(root, "raw", ".json", ident())
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, raw) {
		t.Fatal("bytes changed")
	}
	created, e = w.WriteRaw(ident(), []byte("different"))
	if e != nil || created {
		t.Fatal(created, e)
	}
	got, _ = os.ReadFile(path)
	if !bytes.Equal(got, raw) {
		t.Fatal("duplicate replaced bytes")
	}
}
func TestRawFailureCleansAndDoesNotRetry(t *testing.T) {
	root := t.TempDir()
	enc := &badEncoder{}
	_, e := NewWriter(root, enc).WriteRaw(ident(), nil)
	if e == nil || enc.calls != 1 {
		t.Fatal(e, enc.calls)
	}
	matches, _ := filepath.Glob(filepath.Join(root, "raw", "dt=20260101", "*"))
	if len(matches) != 0 {
		t.Fatalf("left files: %v", matches)
	}
}
func TestProcessedReadBackAndDuplicate(t *testing.T) {
	root := t.TempDir()
	w := NewWriter(root, JSONEncoder{})
	gust := 1.2
	rec := transform.Record{CoordLon: 10.99, CoordLat: 44.34, MainTemp: 20, WindGust: &gust, EventTime: "2026-01-01T00:00:00Z", Weather: []transform.Condition{{ID: 1, Main: "Rain"}}}
	created, e := w.WriteProcessed(ident(), rec)
	if e != nil || !created {
		t.Fatal(created, e)
	}
	path := Path(root, "processed", ".parquet", ident())
	before, _ := os.ReadFile(path)
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	stat, _ := f.Stat()
	reader := parquet.NewGenericReader[transform.Record](f)
	rows := make([]transform.Record, 2)
	n, e := reader.Read(rows)
	if e != nil && e != io.EOF {
		t.Fatal(e)
	}
	if n != 1 || rows[0].EventTime != rec.EventTime || rows[0].CoordLat != 44.34 || rows[0].WindGust == nil || len(rows[0].Weather) != 1 {
		t.Fatalf("rows=%+v size=%d", rows[:n], stat.Size())
	}
	_ = reader.Close()
	created, e = w.WriteProcessed(ident(), transform.Record{EventTime: "changed"})
	if e != nil || created {
		t.Fatal(created, e)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("duplicate replaced parquet")
	}
}
