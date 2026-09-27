# Weather ETL in Go

A small, continuously scheduled service that fetches OpenWeather current conditions, commits the complete response to PostgreSQL, writes the original JSON to a raw zone, and writes one normalized Parquet record to a processed zone. It also serves dependency-aware health and Prometheus endpoints.

## Architecture and guarantees

Each run is serial and has exactly this order:

1. Fetch and validate one OpenWeather response.
2. Insert its complete JSON value in PostgreSQL.
3. Publish the exact response bytes in the raw zone.
4. Transform the typed payload.
5. Publish one Parquet record in the processed zone.

There is **no automatic retry** in the local implementation. A failure stops that run at its current stage; the next timer tick starts an independent run. The database and deterministic files are idempotent, so a later duplicate poll continues downstream and can repair files missing after a partial run. A database failure creates no files; a transform or processed-write failure leaves the committed row and completed raw object available.

## Prerequisites

- Go 1.24 or later
- Docker with Compose v2 for container operation and integration tests
- An OpenWeather current-weather API key (use a non-production key locally)
- `curl` and, for manual database inspection, `psql` or Compose

## Configuration

Copy `.env.example` to `.env`, replace both placeholder secrets, and do not commit it.

| Variable | Required | Default | Meaning |
|---|---:|---|---|
| `OPENWEATHER_API_KEY` | yes | — | OpenWeather API key |
| `WEATHER_LAT` | yes | — | Latitude, from -90 through 90 |
| `WEATHER_LON` | yes | — | Longitude, from -180 through 180 |
| `DATABASE_URL` | yes | — | `postgres://` or `postgresql://` DSN |
| `OPENWEATHER_BASE_URL` | no | `https://api.openweathermap.org/data/2.5/weather` | Current-weather endpoint; override is useful for tests |
| `ETL_INTERVAL` | no | `30s` | Positive Go duration between starts |
| `HTTP_TIMEOUT` | no | `10s` | Positive API/startup timeout |
| `HTTP_ADDR` | no | `:8080` | Monitoring TCP listen address |
| `DATA_DIR` | no | `data` | Root containing `raw` and `processed` |
| `LOG_FILE` | no | `logs/etl.log` | Append-only ETL log path |
| `LOG_STDOUT` | no | `false` | Also mirror logs to stdout when `true` |
| `SHUTDOWN_GRACE` | no | `10s` | Bounded shutdown duration |
| `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` | Compose | — | PostgreSQL container initialization |

Configuration and startup errors never render the API key or DSN. Startup validates values, opens the log, connects and pings PostgreSQL, and applies the embedded migration before scheduling ingestion.

## Run directly with Go

Start PostgreSQL (the profile uses the values in `.env`):

```sh
cp .env.example .env
# Edit .env placeholders first.
docker compose up -d postgres
```

For a host process, load the environment and change the database host from `postgres` to `localhost`:

```sh
set -a; . ./.env; set +a
export DATABASE_URL="postgresql://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:5432/${POSTGRES_DB}?sslmode=disable"
go run ./cmd/weather-etl
```

Stop with SIGINT (`Ctrl-C`) or SIGTERM. The service stops new timer runs, cancels work, shuts down HTTP within `SHUTDOWN_GRACE`, and closes database and log resources.

## Containers and Compose

Build and confirm the image's non-root user:

```sh
docker build -t weather-etl:local .
docker image inspect weather-etl:local --format '{{.Config.User}}'
# Expected: 10001:10001
```

A standalone run needs an existing PostgreSQL network endpoint and writable mounts:

```sh
docker run --rm --env-file .env -p 8080:8080 \
  -v weather-raw:/app/data/raw \
  -v weather-processed:/app/data/processed \
  -v weather-logs:/app/logs weather-etl:local
```

For the complete stack:

```sh
docker compose config
docker compose up --build -d
docker compose ps
```

Compose waits for PostgreSQL's `pg_isready` health check and persists database, raw, processed, and log data in `postgres_data`, `raw_data`, `processed_data`, and `etl_logs`. Named volumes avoid host UID issues. If bind mounts are substituted, make them writable by UID/GID `10001:10001`, for example `sudo chown -R 10001:10001 data logs`.

Restart or recreate only the application without deleting state:

```sh
docker compose restart weather-etl
docker compose up -d --force-recreate weather-etl
```

Rollback by deploying the preceding image tag or stopping the service (`docker compose stop weather-etl`). Existing rows, files, and logs remain compatible evidence. Do **not** use `docker compose down -v` unless all persisted local data may be deleted.

## PostgreSQL storage and inspection

The embedded migration creates:

```sql
CREATE TABLE weather_raw (
  id BIGSERIAL PRIMARY KEY,
  source_dt BIGINT NOT NULL,
  latitude DOUBLE PRECISION NOT NULL,
  longitude DOUBLE PRECISION NOT NULL,
  payload JSONB NOT NULL,
  ingested_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (source_dt, latitude, longitude)
);
```

`source_dt + latitude + longitude` is the logical event identity. Inserts use `ON CONFLICT DO NOTHING`; ingestion timestamps are PostgreSQL `TIMESTAMPTZ` values. Inspect rows with:

```sh
docker compose exec postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -c 'SELECT source_dt, latitude, longitude, ingested_at, payload FROM weather_raw ORDER BY id DESC LIMIT 1;'
```

## Data lake schemas and paths

Partitions use the source event's **UTC** date—not ingestion time or local date. Stable response coordinates and the numeric source `dt` form each name:

```text
data/raw/dt=20260101/1767225600_44.34_10.99.json
data/processed/dt=20260101/1767225600_44.34_10.99.parquet
```

Raw objects contain the exact accepted response bytes, preserving every source field and value. JSON encoding is behind a small encoder boundary so future Avro raw output can be added without changing fetch, path, or pipeline behavior.

Processed output is Parquet only, with exactly one record per object. It contains:

- `coord_lon`, `coord_lat`
- `main_temp`, nullable `main_feels_like`, `main_temp_min`, `main_temp_max`, `main_pressure`, `main_humidity`, `main_sea_level`, `main_grnd_level`
- nullable `wind_speed`, `wind_deg`, `wind_gust`
- UTF-8 `event_time`, converted from source Unix seconds to RFC 3339 UTC (for example `2026-01-01T00:00:00Z`)
- typed retained `weather`, `base`, `visibility`, `rain`, `snow`, `clouds`, `sys`, `timezone`, `id`, `name`, and `cod` columns

Absent optional measurements remain null. Files are written to same-directory temporary objects, synced, and published with no-replace semantics. Reprocessing the same identity leaves existing bytes unchanged and does not increment save metrics. Daily partitions make pruning straightforward; one-file-per-event is intentionally simple for this assignment and requires production compaction.

Locate outputs and logs:

```sh
docker compose exec weather-etl find /app/data -type f -maxdepth 4 -print
docker compose exec weather-etl tail -n 50 /app/logs/etl.log
```

## HTTP endpoints, logs, and metrics

```sh
curl -i http://localhost:8080/healthz
curl -s http://localhost:8080/metrics
```

`GET /healthz` runs a bounded PostgreSQL ping and returns `200 {"status":"ok"}` or `503 {"status":"unhealthy"}`. It is a dependency/readiness check, not a pure liveness probe. `/metrics` returns Prometheus text. Both endpoints reject non-GET methods with 405.

The isolated registry exposes only these application counters:

- `weather_api_requests_total`
- `weather_api_requests_success_total`
- `weather_api_requests_failure_total`
- `etl_transform_total`
- `etl_transform_success_total`
- `etl_transform_errors_total`
- `etl_data_saved_total{destination="postgres|raw|processed"}`

API total equals success plus failure; transform total equals success plus errors. A destination counter increments only for newly saved data, not duplicates. Labels are bounded and never contain coordinates, URLs, errors, or credentials.

Timestamped standard-library logs append to `LOG_FILE`. Events identify API success/failure, transformation success/failure, PostgreSQL/raw/processed success, duplicate, or failure, and service/pipeline state. `LOG_STDOUT=true` mirrors the same safe messages for containers. API keys, DSNs, and credential-bearing request URLs are not logged.

## Validation and tests

From a clean checkout:

```sh
test -z "$(gofmt -l ./cmd ./internal)"
go vet ./...
go test ./...
go build ./...
```

The normal unit suite uses `httptest`, fakes, and temporary directories; it never calls OpenWeather. Run the PostgreSQL integration test against Compose:

```sh
docker compose up -d postgres
set -a; . ./.env; set +a
export TEST_DATABASE_URL="postgresql://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:5432/${POSTGRES_DB}?sslmode=disable"
go test ./internal/postgres -run TestRepositoryIntegration -count=1
```

Container checks:

```sh
docker build -t weather-etl:local .
docker compose config
docker compose up --build -d
```

The lake read-back unit test (`go test ./internal/lake -run TestProcessedReadBackAndDuplicate -count=1`) verifies one readable Parquet row, its ISO timestamp, nested/nullable fields, and no-replace behavior.

## Troubleshooting

- **Configuration error:** compare `.env` with the table; durations must be positive and URLs/coordinates valid.
- **Startup `ping PostgreSQL`:** wait for `docker compose ps` to show PostgreSQL healthy and ensure host is `postgres` in Compose or `localhost` for direct Go.
- **`/healthz` is 503:** PostgreSQL is unavailable or exceeded the bounded ping timeout; inspect `docker compose logs postgres`.
- **No new output:** OpenWeather often repeats `dt`; duplicates are expected. Inspect metrics and `logs/etl.log` for a stage-specific result.
- **Permission denied:** use named volumes or grant UID/GID 10001 ownership on bind-mounted data/log directories.
- **Partial event:** fix the failed dependency and wait for the same event or replay it externally; idempotent stages repair missing downstream output. The service does not retry automatically.
- **Rate limits/timeouts:** the run ends after one attempt; increase `ETL_INTERVAL` or `HTTP_TIMEOUT` as appropriate.

## Design rationale and productionization

The single Go process and narrow packages keep the assignment testable without a workflow framework. PostgreSQL is the first durable boundary, preserving complete JSONB before files. A transaction cannot atomically cover PostgreSQL and a filesystem, so strict ordering plus deterministic identities provides recoverability rather than claiming distributed exactly-once delivery. Parquet supports analytical scans; JSON remains inspectable source evidence.

A production AWS evolution would use:

- EKS or ECS/Fargate for controlled worker and endpoint deployment; EventBridge Scheduler or partition ownership/leader election ensures one owner per location/time slice.
- Secrets Manager for API/database credentials, KMS encryption, private networking, TLS, least-privilege IAM, image scanning, and audited access.
- Versioned, encrypted S3 raw/processed zones with conditional object creation; RDS PostgreSQL Multi-AZ for relational durability; Glue Data Catalog and Athena for discovery/query.
- SQS/Kinesis between ingestion and transformation so stages scale independently. Event IDs remain idempotency keys. Production consumers use **bounded** exponential backoff with jitter for transient failures, then a DLQ; controlled replay preserves identity and does not overwrite objects. These retries are production-only—the local service has none.
- Explicit schema versions, compatibility checks, Glue schema/catalog evolution, quarantine for incompatible events, and data-quality checks for freshness, ranges, null rates, cardinality, and raw/processed reconciliation.
- CloudWatch logs/alarms plus Managed Prometheus and Grafana for availability, lag, failures, DLQ depth, latency, saturation, and last-success signals. (The local metric surface intentionally remains requirement-limited.)
- Scheduled Parquet compaction to address small files, S3 lifecycle retention/tiering, and legal deletion policies. RDS point-in-time recovery, automated snapshots, cross-region snapshot/S3 replication, and tested restores provide backup and disaster recovery.
- Multi-AZ compute and RDS, health-based replacement, queue buffering, deployment disruption budgets, and documented RTO/RPO with regular failover and regional recovery exercises.

This separates scaling and ownership, preserves replayability, and improves security/availability without suggesting that production retry machinery exists in this local take-home implementation.
