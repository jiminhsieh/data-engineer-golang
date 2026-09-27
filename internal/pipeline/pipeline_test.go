package pipeline

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"

	"github.com/jiminhsieh/data-engineer-golang/internal/lake"
	"github.com/jiminhsieh/data-engineer-golang/internal/observability"
	"github.com/jiminhsieh/data-engineer-golang/internal/transform"
	"github.com/jiminhsieh/data-engineer-golang/internal/weather"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type fakeFetch struct {
	order *[]string
	err   error
	calls int
}

func (f *fakeFetch) Fetch(context.Context) (weather.Result, error) {
	f.calls++
	*f.order = append(*f.order, "fetch")
	if f.err != nil {
		return weather.Result{}, f.err
	}
	temp := 1.0
	v := weather.Current{DT: 1767225600, Coord: &weather.Coordinates{Latitude: 44.34, Longitude: 10.99}, Main: &weather.Main{Temp: &temp}}
	return weather.Result{Raw: []byte(`{"ok":true}`), Weather: v}, nil
}

type fakeRepo struct {
	order   *[]string
	err     error
	created bool
	calls   int
}

func (f *fakeRepo) Insert(context.Context, int64, float64, float64, []byte) (bool, error) {
	f.calls++
	*f.order = append(*f.order, "postgres")
	return f.created, f.err
}

type fakeLake struct {
	order                        *[]string
	rawErr, processedErr         error
	rawCreated, processedCreated bool
	rawCalls, processedCalls     int
}

func (f *fakeLake) WriteRaw(lake.Identity, []byte) (bool, error) {
	f.rawCalls++
	*f.order = append(*f.order, "raw")
	return f.rawCreated, f.rawErr
}
func (f *fakeLake) WriteProcessed(lake.Identity, transform.Record) (bool, error) {
	f.processedCalls++
	*f.order = append(*f.order, "processed")
	return f.processedCreated, f.processedErr
}
func setup(fail string, duplicates bool) (*Pipeline, *fakeFetch, *fakeRepo, *fakeLake, *observability.Metrics, *[]string, *bytes.Buffer) {
	order := []string{}
	fetch := &fakeFetch{order: &order}
	repo := &fakeRepo{order: &order, created: !duplicates}
	store := &fakeLake{order: &order, rawCreated: !duplicates, processedCreated: !duplicates}
	fn := Transform(func(w weather.Current) (transform.Record, error) {
		order = append(order, "transform")
		if fail == "transform" {
			return transform.Record{}, errors.New("transform failed")
		}
		return transform.Weather(w)
	})
	switch fail {
	case "api":
		fetch.err = errors.New("api failed")
	case "postgres":
		repo.err = errors.New("db failed")
	case "raw":
		store.rawErr = errors.New("raw failed")
	case "processed":
		store.processedErr = errors.New("parquet failed")
	}
	m := observability.NewMetrics()
	buf := &bytes.Buffer{}
	return New(fetch, repo, store, fn, log.New(buf, "", 0), m), fetch, repo, store, m, &order, buf
}
func TestPipelineOrderAndMetrics(t *testing.T) {
	p, _, _, _, m, order, _ := setup("", false)
	if e := p.Run(context.Background()); e != nil {
		t.Fatal(e)
	}
	if strings.Join(*order, ",") != "fetch,postgres,raw,transform,processed" {
		t.Fatal(*order)
	}
	for _, c := range []struct {
		name string
		v    float64
	}{{"api total", testutil.ToFloat64(m.APITotal)}, {"api ok", testutil.ToFloat64(m.APISuccess)}, {"transform total", testutil.ToFloat64(m.TransformTotal)}, {"transform ok", testutil.ToFloat64(m.TransformSuccess)}} {
		if c.v != 1 {
			t.Fatal(c.name, c.v)
		}
	}
	if testutil.ToFloat64(m.DataSaved.WithLabelValues("postgres")) != 1 || testutil.ToFloat64(m.DataSaved.WithLabelValues("raw")) != 1 || testutil.ToFloat64(m.DataSaved.WithLabelValues("processed")) != 1 {
		t.Fatal("save metrics")
	}
}
func TestPipelineFailureShortCircuits(t *testing.T) {
	expected := map[string]string{"api": "fetch", "postgres": "fetch,postgres", "raw": "fetch,postgres,raw", "transform": "fetch,postgres,raw,transform", "processed": "fetch,postgres,raw,transform,processed"}
	for stage, want := range expected {
		t.Run(stage, func(t *testing.T) {
			p, f, r, l, m, order, logs := setup(stage, false)
			if e := p.Run(context.Background()); e == nil {
				t.Fatal("wanted error")
			}
			if strings.Join(*order, ",") != want {
				t.Fatalf("%v", *order)
			}
			if f.calls > 1 || r.calls > 1 || l.rawCalls > 1 || l.processedCalls > 1 {
				t.Fatal("retried")
			}
			if !strings.Contains(logs.String(), "outcome=failure") {
				t.Fatal(logs.String())
			}
			if stage == "api" && testutil.ToFloat64(m.APIFailure) != 1 {
				t.Fatal("failure metric")
			}
			if stage == "transform" && testutil.ToFloat64(m.TransformErrors) != 1 {
				t.Fatal("transform metric")
			}
		})
	}
}
func TestDuplicatesContinueWithoutSaveMetrics(t *testing.T) {
	p, _, _, _, m, order, _ := setup("", true)
	if e := p.Run(context.Background()); e != nil {
		t.Fatal(e)
	}
	if len(*order) != 5 {
		t.Fatal(*order)
	}
	for _, d := range []string{"postgres", "raw", "processed"} {
		if testutil.ToFloat64(m.DataSaved.WithLabelValues(d)) != 0 {
			t.Fatal(d)
		}
	}
}
