package config

import (
	"strings"
	"testing"
	"time"
)

func env(values map[string]string) LookupEnv {
	return func(k string) (string, bool) { v, ok := values[k]; return v, ok }
}

func validEnv() map[string]string {
	return map[string]string{
		"OPENWEATHER_API_KEY": "secret", "WEATHER_LAT": "44.34", "WEATHER_LON": "10.99",
		"DATABASE_URL": "postgres://user:password@localhost/weather?sslmode=disable",
	}
}

func TestDefaults(t *testing.T) {
	cfg, err := FromEnvironment(env(validEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Interval != 30*time.Second || cfg.HTTPTimeout != 10*time.Second || cfg.HTTPAddr != ":8080" || cfg.DataDir != "data" || cfg.LogFile != "logs/etl.log" || cfg.APIBaseURL != DefaultAPIBaseURL {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestInvalidConfiguration(t *testing.T) {
	tests := []struct{ name, key, value string }{
		{"missing key", "OPENWEATHER_API_KEY", ""}, {"bad lat", "WEATHER_LAT", "91"}, {"nan lat", "WEATHER_LAT", "NaN"},
		{"bad lon", "WEATHER_LON", "-181"}, {"bad duration", "ETL_INTERVAL", "0s"},
		{"bad timeout", "HTTP_TIMEOUT", "wat"}, {"bad addr", "HTTP_ADDR", "not an address"},
		{"bad api url", "OPENWEATHER_BASE_URL", "ftp://example.com"}, {"bad db", "DATABASE_URL", "https://example.com"},
		{"bad data path", "DATA_DIR", "\x00"}, {"bad log path", "LOG_FILE", "\x00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := validEnv()
			v[tt.key] = tt.value
			_, err := FromEnvironment(env(v))
			if err == nil {
				t.Fatal("wanted error")
			}
			if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "password") {
				t.Fatalf("secret leaked: %v", err)
			}
		})
	}
}
