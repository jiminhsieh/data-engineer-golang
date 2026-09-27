package lake

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jiminhsieh/data-engineer-golang/internal/transform"
	"github.com/parquet-go/parquet-go"
)

type Identity struct {
	SourceDT            int64
	Latitude, Longitude float64
}
type Encoder interface {
	Extension() string
	Encode(io.Writer, []byte) error
}
type JSONEncoder struct{}

func (JSONEncoder) Extension() string                  { return ".json" }
func (JSONEncoder) Encode(w io.Writer, b []byte) error { _, e := w.Write(b); return e }

type Writer struct {
	root string
	raw  Encoder
}

func NewWriter(root string, raw Encoder) *Writer { return &Writer{root: root, raw: raw} }

func Path(root, zone, extension string, id Identity) string {
	date := time.Unix(id.SourceDT, 0).UTC().Format("20060102")
	name := strconv.FormatInt(id.SourceDT, 10) + "_" + number(id.Latitude) + "_" + number(id.Longitude) + extension
	return filepath.Join(root, zone, "dt="+date, name)
}
func number(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
func (w *Writer) WriteRaw(id Identity, raw []byte) (bool, error) {
	if w.raw == nil {
		return false, errors.New("raw encoder is required")
	}
	target := Path(w.root, "raw", w.raw.Extension(), id)
	return publish(target, func(out io.Writer) error { return w.raw.Encode(out, raw) })
}
func (w *Writer) WriteProcessed(id Identity, record transform.Record) (bool, error) {
	target := Path(w.root, "processed", ".parquet", id)
	return publish(target, func(out io.Writer) error {
		pw := parquet.NewGenericWriter[transform.Record](out)
		if _, err := pw.Write([]transform.Record{record}); err != nil {
			return err
		}
		return pw.Close()
	})
}

func publish(target string, encode func(io.Writer) error) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return false, fmt.Errorf("create partition: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".etl-*.tmp")
	if err != nil {
		return false, fmt.Errorf("create temporary object: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err = encode(tmp); err != nil {
		_ = tmp.Close()
		return false, fmt.Errorf("encode object: %w", err)
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return false, fmt.Errorf("sync object: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return false, fmt.Errorf("close object: %w", err)
	}
	if err = os.Link(tmpName, target); err != nil {
		if errors.Is(err, os.ErrExist) {
			return false, nil
		}
		return false, fmt.Errorf("publish object: %w", err)
	}
	return true, nil
}
