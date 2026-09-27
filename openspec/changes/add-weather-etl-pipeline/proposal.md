# Proposal

## Why

The repository currently documents a take-home ETL assignment but has no executable pipeline. A complete Go service is needed to ingest OpenWeather current-weather data, persist raw and processed representations, and expose enough operational signals to run and assess it locally or in a container.

## What Changes

- Add a configurable, 30-second weather ingestion loop that makes one OpenWeather request attempt per scheduled run and stores each accepted raw response as PostgreSQL JSONB.
- Add raw JSON and processed Parquet data-lake outputs, organized in UTC event-date partitions and written without overwriting existing objects.
- Add transformation from the OpenWeather payload to a stable flattened record that renames source `dt` to `event_time` and converts it from Unix seconds to an ISO 8601 UTC string.
- Add file logging, health and Prometheus metrics HTTP endpoints, including signals for requests, runs, transformations, writes, latency, and last success.
- Add container packaging, local PostgreSQL orchestration, persistent-volume instructions, automated tests for key logic, and operational/production guidance.

## Capabilities

### New Capabilities
- `weather-ingestion`: Scheduled OpenWeather extraction, configuration, validation, and raw JSONB persistence in PostgreSQL.
- `weather-data-lake`: Raw JSON export, weather transformation, processed Parquet export, partitioning, naming, and no-overwrite behavior.
- `pipeline-observability`: Required ETL file logs plus health and Prometheus-compatible metrics endpoints.
- `containerized-operation`: Reproducible container execution, persistent mounts, database orchestration, and operator documentation.

### Modified Capabilities

None.

## Impact

This creates the Go application and tests, PostgreSQL schema/migrations, local data and log directories, Docker assets, configuration examples, and expanded README documentation. Runtime integrations are the OpenWeather API, PostgreSQL, the local filesystem, and Prometheus-compatible monitoring. New Go dependencies will cover PostgreSQL access, Prometheus instrumentation, and Parquet encoding.
