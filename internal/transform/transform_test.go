package transform

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jiminhsieh/data-engineer-golang/internal/weather"
)

func load(t *testing.T, name string) weather.Current {
	t.Helper()
	b, e := os.ReadFile(filepath.Join("..", "..", "testdata", name))
	if e != nil {
		t.Fatal(e)
	}
	v, e := weather.Decode(b)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func TestWeatherComplete(t *testing.T) {
	r, e := Weather(load(t, "weather_complete.json"))
	if e != nil {
		t.Fatal(e)
	}
	if r.EventTime != "2026-01-01T00:00:00Z" || r.CoordLat != 44.34 || r.MainSeaLevel == nil || r.WindGust == nil || len(r.Weather) != 1 || r.Sys == nil || r.Cod == nil {
		t.Fatalf("bad record: %+v", r)
	}
}
func TestWeatherOptionalsRemainNil(t *testing.T) {
	r, e := Weather(load(t, "weather_optional.json"))
	if e != nil {
		t.Fatal(e)
	}
	if r.WindGust != nil || r.MainSeaLevel != nil || r.Rain != nil || r.Snow != nil {
		t.Fatalf("invented optional: %+v", r)
	}
}
func TestWeatherRejectsInvalidRequiredValue(t *testing.T) {
	v := load(t, "weather_optional.json")
	v.DT = 0
	if _, e := Weather(v); e == nil {
		t.Fatal("wanted error")
	}
	v = load(t, "weather_optional.json")
	v.Cod = []byte(`"200"`)
	if _, e := Weather(v); e == nil {
		t.Fatal("wanted incompatible cod error")
	}
}
