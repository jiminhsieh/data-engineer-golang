package transform

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jiminhsieh/data-engineer-golang/internal/weather"
)

type Condition struct {
	ID          int64  `parquet:"id"`
	Main        string `parquet:"main"`
	Description string `parquet:"description"`
	Icon        string `parquet:"icon"`
}
type Precipitation struct {
	OneHour *float64 `parquet:"one_hour,optional"`
}
type Clouds struct {
	All *int64 `parquet:"all,optional"`
}
type System struct {
	Type    *int64  `parquet:"type,optional"`
	ID      *int64  `parquet:"id,optional"`
	Country *string `parquet:"country,optional"`
	Sunrise *int64  `parquet:"sunrise,optional"`
	Sunset  *int64  `parquet:"sunset,optional"`
}

type Record struct {
	CoordLon        float64        `parquet:"coord_lon"`
	CoordLat        float64        `parquet:"coord_lat"`
	MainTemp        float64        `parquet:"main_temp"`
	MainFeelsLike   *float64       `parquet:"main_feels_like,optional"`
	MainTempMin     *float64       `parquet:"main_temp_min,optional"`
	MainTempMax     *float64       `parquet:"main_temp_max,optional"`
	MainPressure    *int64         `parquet:"main_pressure,optional"`
	MainHumidity    *int64         `parquet:"main_humidity,optional"`
	MainSeaLevel    *int64         `parquet:"main_sea_level,optional"`
	MainGroundLevel *int64         `parquet:"main_grnd_level,optional"`
	WindSpeed       *float64       `parquet:"wind_speed,optional"`
	WindDegrees     *int64         `parquet:"wind_deg,optional"`
	WindGust        *float64       `parquet:"wind_gust,optional"`
	EventTime       string         `parquet:"event_time"`
	Weather         []Condition    `parquet:"weather,list"`
	Base            *string        `parquet:"base,optional"`
	Visibility      *int64         `parquet:"visibility,optional"`
	Rain            *Precipitation `parquet:"rain,optional"`
	Snow            *Precipitation `parquet:"snow,optional"`
	Clouds          *Clouds        `parquet:"clouds,optional"`
	Sys             *System        `parquet:"sys,optional"`
	Timezone        *int64         `parquet:"timezone,optional"`
	ID              *int64         `parquet:"id,optional"`
	Name            *string        `parquet:"name,optional"`
	Cod             *int64         `parquet:"cod,optional"`
}

func Weather(input weather.Current) (Record, error) {
	if input.DT <= 0 || input.Coord == nil || math.IsNaN(input.Coord.Latitude) || math.IsInf(input.Coord.Latitude, 0) || math.IsNaN(input.Coord.Longitude) || math.IsInf(input.Coord.Longitude, 0) || input.Coord.Latitude < -90 || input.Coord.Latitude > 90 || input.Coord.Longitude < -180 || input.Coord.Longitude > 180 {
		return Record{}, errors.New("invalid event identity")
	}
	if input.Main == nil || input.Main.Temp == nil {
		return Record{}, errors.New("main temperature is required")
	}
	var cod *int64
	if len(input.Cod) > 0 && string(input.Cod) != "null" {
		var n int64
		if err := json.Unmarshal(input.Cod, &n); err != nil {
			return Record{}, fmt.Errorf("cod must be numeric: %w", err)
		}
		cod = &n
	}
	r := Record{CoordLon: input.Coord.Longitude, CoordLat: input.Coord.Latitude, MainTemp: *input.Main.Temp, MainFeelsLike: input.Main.FeelsLike, MainTempMin: input.Main.TempMin, MainTempMax: input.Main.TempMax, MainPressure: input.Main.Pressure, MainHumidity: input.Main.Humidity, MainSeaLevel: input.Main.SeaLevel, MainGroundLevel: input.Main.GroundLevel, EventTime: time.Unix(input.DT, 0).UTC().Format(time.RFC3339), Visibility: input.Visibility, Timezone: input.Timezone, ID: input.ID, Name: input.Name, Base: input.Base, Cod: cod}
	if input.Wind != nil {
		r.WindSpeed = input.Wind.Speed
		r.WindDegrees = input.Wind.Degrees
		r.WindGust = input.Wind.Gust
	}
	for _, w := range input.Weather {
		r.Weather = append(r.Weather, Condition{ID: w.ID, Main: w.Main, Description: w.Description, Icon: w.Icon})
	}
	if input.Rain != nil {
		r.Rain = &Precipitation{OneHour: input.Rain.OneHour}
	}
	if input.Snow != nil {
		r.Snow = &Precipitation{OneHour: input.Snow.OneHour}
	}
	if input.Clouds != nil {
		r.Clouds = &Clouds{All: input.Clouds.All}
	}
	if input.Sys != nil {
		r.Sys = &System{Type: input.Sys.Type, ID: input.Sys.ID, Country: input.Sys.Country, Sunrise: input.Sys.Sunrise, Sunset: input.Sys.Sunset}
	}
	return r, nil
}
