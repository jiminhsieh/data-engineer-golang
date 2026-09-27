# Implementation Decision
This is the companion doc for the take-home assignment PDF so agent can have details to implement.

## Open API
We're using [current weather data API](https://openweathermap.org/api/current?collection=current_forecast) from OpenWeather.

## How we store JSON response into PostgreSQL 
After we get JSON, we store raw format as JSONB in PostgreSQL

## Data Lake Design
1. Need to have a dt directory under raw and processed zones. The directory name is `dt={yyyymmdd}` like `dt=20260101`. The files will be placed under correct dt dir.
2. The value of dt in directory will come from dt field in the API response.
3. No matter what's the data format, the file name should be {dt_from_JSON_top-level-field}_{lat}_{lon}.<file_extension>. Below is the example
```
data/raw/dt=20260101/1767225600_44.34_10.99.json
data/processed/dt=20260101/1767225600_44.34_10.99.parquet
```

## How we store raw data in `data/raw/`
We store the original raw data in JSON format, but the code design needs to consider that it could be changed to AVRO. 

## How we store transformed data in `data/processed/`
We store transformed data in Parquet format.

## Transformation Logic
This is for processed zone. 

1. Below top-level JSON objects need to be flattened 
   1. coord will become to coord_lon and coord_lat
   2. main
   3. wind
2. Change the raw JSON response's `dt` field name to `event_time` and convert data from epoch to ISO 8601 timestamp format


## Code Design
1. Follow KISS.
2. Keep logging and metrics limited to what the [spec](take-home_assignment.md) defines.
3. Consider unit tests for key logics from the [spec](take-home_assignment.md) and Transformation Logic section.
4. Skip retries for now since they aren't specified in the [spec](take-home_assignment.md).
