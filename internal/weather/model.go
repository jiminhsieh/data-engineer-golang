package weather

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
)

type Coordinates struct {
	Longitude float64 `json:"lon" parquet:"lon"`
	Latitude  float64 `json:"lat" parquet:"lat"`
}
type Condition struct {
	ID          int64  `json:"id" parquet:"id"`
	Main        string `json:"main" parquet:"main"`
	Description string `json:"description" parquet:"description"`
	Icon        string `json:"icon" parquet:"icon"`
}
type Main struct {
	Temp        *float64 `json:"temp"`
	FeelsLike   *float64 `json:"feels_like,omitempty"`
	TempMin     *float64 `json:"temp_min,omitempty"`
	TempMax     *float64 `json:"temp_max,omitempty"`
	Pressure    *int64   `json:"pressure,omitempty"`
	Humidity    *int64   `json:"humidity,omitempty"`
	SeaLevel    *int64   `json:"sea_level,omitempty"`
	GroundLevel *int64   `json:"grnd_level,omitempty"`
}
type Wind struct {
	Speed   *float64 `json:"speed,omitempty"`
	Degrees *int64   `json:"deg,omitempty"`
	Gust    *float64 `json:"gust,omitempty"`
}
type Precipitation struct {
	OneHour *float64 `json:"1h,omitempty"`
}
type Clouds struct {
	All *int64 `json:"all,omitempty" parquet:"all,optional"`
}
type System struct {
	Type    *int64  `json:"type,omitempty" parquet:"type,optional"`
	ID      *int64  `json:"id,omitempty" parquet:"id,optional"`
	Country *string `json:"country,omitempty" parquet:"country,optional"`
	Sunrise *int64  `json:"sunrise,omitempty" parquet:"sunrise,optional"`
	Sunset  *int64  `json:"sunset,omitempty" parquet:"sunset,optional"`
}

type Current struct {
	Coord      *Coordinates    `json:"coord"`
	Weather    []Condition     `json:"weather,omitempty"`
	Base       *string         `json:"base,omitempty"`
	Main       *Main           `json:"main"`
	Visibility *int64          `json:"visibility,omitempty"`
	Wind       *Wind           `json:"wind,omitempty"`
	Rain       *Precipitation  `json:"rain,omitempty"`
	Snow       *Precipitation  `json:"snow,omitempty"`
	Clouds     *Clouds         `json:"clouds,omitempty"`
	DT         int64           `json:"dt"`
	Sys        *System         `json:"sys,omitempty"`
	Timezone   *int64          `json:"timezone,omitempty"`
	ID         *int64          `json:"id,omitempty"`
	Name       *string         `json:"name,omitempty"`
	Cod        json.RawMessage `json:"cod,omitempty"`
}

func Decode(raw []byte) (Current, error) {
	var value Current
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&value); err != nil {
		return Current{}, fmt.Errorf("decode weather response: %w", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return Current{}, errors.New("decode weather response: multiple JSON values")
		}
		return Current{}, fmt.Errorf("decode weather response trailing data: %w", err)
	}
	if value.DT <= 0 {
		return Current{}, errors.New("weather response has invalid source dt")
	}
	if value.Coord == nil || math.IsNaN(value.Coord.Latitude) || math.IsInf(value.Coord.Latitude, 0) || math.IsNaN(value.Coord.Longitude) || math.IsInf(value.Coord.Longitude, 0) || value.Coord.Latitude < -90 || value.Coord.Latitude > 90 || value.Coord.Longitude < -180 || value.Coord.Longitude > 180 {
		return Current{}, errors.New("weather response has invalid coordinates")
	}
	if value.Main == nil || value.Main.Temp == nil {
		return Current{}, errors.New("weather response has no required main temperature")
	}
	return value, nil
}
