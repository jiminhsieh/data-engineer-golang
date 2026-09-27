package weather

import (
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDecode(t *testing.T) {
	full, err := Decode(fixture(t, "weather_complete.json"))
	if err != nil {
		t.Fatal(err)
	}
	if full.DT != 1767225600 || full.Wind == nil || full.Wind.Gust == nil || len(full.Weather) != 1 {
		t.Fatalf("incomplete decode: %+v", full)
	}
	optional, err := Decode(fixture(t, "weather_optional.json"))
	if err != nil {
		t.Fatal(err)
	}
	if optional.Wind != nil || optional.Main.SeaLevel != nil {
		t.Fatal("absent values should remain nil")
	}
}

func TestDecodeRejectsInvalid(t *testing.T) {
	for _, raw := range [][]byte{[]byte(`{`), fixture(t, "weather_error.json"), []byte(`{"coord":{"lat":99,"lon":1},"main":{"temp":2},"dt":1}`), []byte(`{"coord":{"lat":1,"lon":1},"main":null,"dt":1}`)} {
		if _, err := Decode(raw); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
