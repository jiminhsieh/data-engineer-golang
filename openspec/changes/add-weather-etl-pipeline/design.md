# Design

## Context

See `proposal.md` for motivation and the capability specs for behavior. The repository is currently documentation-only: there is no Go module, application, database schema, test harness, or container definition to preserve. The service must combine a continuously scheduled worker with an HTTP monitoring surface and must commit one event across PostgreSQL and two local filesystem zones without a distributed transaction.

The implementation decision defines a UTC daily partition rendered as `dt=YYYYMMDD`, for example `dt=20260101`. Partition paths and filenames use the response's numeric source `dt` and response coordinates, not transformed `event_time`, wall-clock ingestion time, or configured coordinate text.

## Goals / Non-Goals

**Goals:**
- Keep stage boundaries small and testable while retaining the original response separately from typed processing models.
- Make repeated OpenWeather source timestamps idempotent across PostgreSQL and local files without adding automatic retry machinery.
- Permit raw JSON to be replaced by another encoder later without coupling that choice to fetching, transformation, or path construction.
- Run the worker and monitoring server in one simple process with graceful cancellation.
- Provide deterministic local/container execution and enough documentation to explain a production evolution.

**Non-Goals:**
- Exactly-once transactions spanning PostgreSQL and the filesystem.
- In-process backfill, distributed scheduling, automatic compaction, schema registry integration, or support for multiple locations in one process.
- Automatic retries for API, PostgreSQL, transformation, or file failures; a failure ends the current run and the next timer event starts an independent run.
- A general-purpose workflow framework or pluggable system for every component.
- Production cloud infrastructure as code.

## Decisions

### 1. One composed binary with narrow internal packages

Use `cmd/weather-etl` as the composition root and focused packages under `internal/` for configuration, OpenWeather HTTP/model handling, PostgreSQL persistence, transformation, lake paths/writers, pipeline orchestration, and observability. Interfaces will be introduced only at network, database, and encoding/storage boundaries that tests or the stated future Avro option need.

The process starts the metrics/health server and a serial scheduler under one root context. The scheduler runs immediately, then on a ticker; it never launches overlapping goroutines. SIGINT/SIGTERM cancellation stops the ticker and initiates bounded HTTP and pipeline shutdown.

*Alternatives considered:* Separate ingestion, transformation, and API services would improve independent scaling but introduce queues, deployment coordination, and distributed failure modes that are disproportionate for this assignment. A single `main` package would be shorter initially but make stage behavior and tests harder to isolate.

### 2. Environment configuration with explicit validation

Configuration will be parsed once from environment variables:

- Required: `OPENWEATHER_API_KEY`, `WEATHER_LAT`, `WEATHER_LON`, `DATABASE_URL`.
- Defaulted: `ETL_INTERVAL=30s`, `HTTP_TIMEOUT=10s`, `HTTP_ADDR=:8080`, `DATA_DIR=data`, `LOG_FILE=logs/etl.log`, and the HTTPS OpenWeather current-weather base URL.
- Test-only/local override: the API base URL can be changed to an `httptest` server through the same parser while the production default remains HTTPS.

Parsing validates coordinate ranges, positive durations, valid URL/address/path values, opens the append-only log, connects to PostgreSQL, and applies the embedded migration before scheduling begins. Secret-bearing values are never rendered in errors or logs.

*Alternatives considered:* Flags or a configuration file add precedence and secret-mount concerns with no benefit for a container-first single process. Defaults for coordinates were rejected because location is business input, not an implementation default.

### 3. Preserve raw bytes and use typed models for validation and Parquet

The OpenWeather client returns the response bytes and a decoded current-weather model. It limits response size, rejects non-2xx responses, makes no automatic retries, decodes JSON strictly enough to validate required source `dt`/coordinates, and leaves documented optional fields nullable. The exact successful body is passed to PostgreSQL and the raw JSON writer; PostgreSQL JSONB preserves the complete JSON value while the raw zone preserves the source representation.

Transformation maps to a separate typed processed record. `coord`, `main`, and `wind` members become prefixed scalar columns, source `dt` becomes `event_time` formatted as an RFC 3339 ISO 8601 UTC string (for example, `2026-01-01T00:00:00Z`), and every other source field is retained using typed scalar, nested, or nullable columns. Keeping API and processed models separate prevents storage tags and flattening concerns from contaminating extraction.

Use `github.com/parquet-go/parquet-go` for one-record Parquet files, with `event_time` represented as a UTF-8 string. Parquet is the only processed format. A minimal raw-encoder interface supplies a format extension and writes bytes; JSON is the only raw implementation now. Path calculation is format-independent, so an Avro encoder can later replace raw JSON without changing pipeline flow.

*Alternatives considered:* Dynamic maps preserve unknown future fields but do not provide a stable, typed Parquet schema. Storing nested values as JSON strings is simpler but weakens analytical value. Flattening every nested object was rejected because only `coord`, `main`, and `wind` are called out as common query dimensions.

### 4. PostgreSQL is the first durable boundary

An embedded SQL migration creates a `weather_raw` table with a generated ID, `source_dt BIGINT`, `latitude` and `longitude` numeric columns, `payload JSONB`, and `ingested_at TIMESTAMPTZ`. A unique constraint on source `dt` and coordinates represents the logical event identity. Inserts use `ON CONFLICT DO NOTHING` and report created versus duplicate without hiding other database errors or retrying failed statements.

A run executes in this order:

1. Fetch and validate the response.
2. Upsert the complete raw response into PostgreSQL.
3. Write or confirm the raw JSON object.
4. Transform the typed response.
5. Write or confirm the processed Parquet object.
6. Mark the run fully successful.

Downstream steps still run after a duplicate database result, allowing a repeat poll to repair missing files from an earlier partial run. A database failure prevents filesystem writes. Raw output remains available if later transformation or Parquet writing fails.

*Alternatives considered:* A transaction cannot atomically include local files. Writing files first can produce events that never reach the required PostgreSQL store. An outbox/reconciler would close the recovery gap but is beyond the KISS scope; production guidance will recommend decoupled, replayable processing.

### 5. Daily UTC partitions and no-replace publishing

A pure path builder converts numeric source `dt` with `time.Unix(...).UTC()` and Go layout `20060102` to render `YYYYMMDD`. Stable decimal formatting of response latitude and longitude produces:

```text
<data-root>/raw/dt=YYYYMMDD/<source_dt>_<lat>_<lon>.json
<data-root>/processed/dt=YYYYMMDD/<source_dt>_<lat>_<lon>.parquet
```

Each writer creates its partition recursively, writes to a same-directory temporary file, closes/syncs it, and publishes it with no-replace semantics. If the final path already exists, the temporary file is removed and the result is classified as an idempotent duplicate; no existing bytes are opened for writing. Normal write/encode failures remove temporary artifacts. Unit tests verify the existing file remains byte-for-byte unchanged.

*Alternatives considered:* Appending multiple JSON values to one daily file conflicts with valid standalone JSON and is unsafe for concurrent readers. A random suffix avoids collisions but violates deterministic naming and weakens idempotency. Ordinary rename was rejected because it can overwrite its destination.

### 6. Standard logging plus bounded Prometheus labels

Create one `log.Logger` backed by an append-opened `logs/etl.log`; optionally mirror to stdout for container diagnostics through `io.MultiWriter` without introducing a logging framework. Messages use stable key-value text (`stage`, `outcome`, event identity when known, and error) and are emitted at each required stage. API keys, DSNs, and request URLs containing credentials are never logged.

Use the Prometheus Go client and a dedicated registry with these application metrics:

- `weather_api_requests_total{outcome}`
- `etl_runs_total{outcome}`
- `etl_run_duration_seconds`
- `etl_transform_errors_total`
- `etl_storage_operations_total{destination,outcome}` for `postgres`, `raw`, and `processed`
- `etl_storage_errors_total{destination}`
- `etl_last_success_unixtime`

Only fixed enumerations appear in labels. `/metrics` uses `promhttp`; `/healthz` performs a short context-bounded PostgreSQL ping and emits minimal JSON. Method guards return 405. HTTP handler tests use an isolated registry to avoid global collector collisions.

*Alternatives considered:* Only logs do not support alerting or rate/duration analysis. Per-location and per-error metric labels were rejected because they create cardinality and disclosure risks. A structured logging dependency would be unnecessary for the assignment.

### 7. Container, orchestration, and test strategy

The Dockerfile uses a pinned Go builder stage and a small runtime stage with CA certificates, a non-root user, and writable `/app/data` and `/app/logs` directories. A `.dockerignore` keeps credentials, generated data, logs, and local artifacts out of the build context. `compose.yaml` starts PostgreSQL with a health check, persists its state, waits for health before starting ETL, and bind- or volume-mounts the two application paths. `.env.example` contains names and safe defaults only.

Tests use table-driven unit tests, `httptest`, temporary directories, controllable clocks where path dates matter, and isolated Prometheus registries. PostgreSQL behavior is covered by an integration test against a documented `TEST_DATABASE_URL`; the local orchestration/CI test command supplies PostgreSQL. Tests do not call the real OpenWeather API. The validation sequence is format check, `go vet`, `go test`, and `go build`, followed by container build and a compose smoke test.

*Alternatives considered:* Mocking SQL verifies calls but not JSONB, uniqueness, or migration behavior. Starting a hidden test container from every `go test` invocation adds Docker coupling and a large helper dependency; an explicit integration database keeps that dependency visible.

### 8. Documentation treats local files as a simulation, not the production design

The README will replace the current partial notes with runnable instructions, exact schemas/layout, endpoint and metric explanations, mount examples, failure behavior, the local implementation's explicit lack of automatic retries, and design rationale. Its production section will map the worker to a controlled EKS/ECS deployment, secrets to Secrets Manager, raw/processed zones to versioned S3, PostgreSQL to RDS, metadata to Glue Catalog, queries to Athena, and telemetry to CloudWatch/Managed Prometheus and Grafana. Production guidance will describe queue-based decoupling, partition ownership/leader election, bounded retries with backoff and jitter, DLQs/replay, compaction, schema evolution, quality checks, encryption/IAM, retention, backups, multi-AZ operation, and recovery testing.

## Risks / Trade-offs

- [OpenWeather commonly returns the same `dt` across several 30-second polls] → Use the event identity unique key and deterministic files; classify repeats as successful idempotent duplicates.
- [A crash between PostgreSQL, raw, and processed commits leaves a partial event] → Preserve strict stage ordering and idempotent reruns; document database-driven replay/outbox processing as the production solution.
- [One file per event creates a small-file problem at scale] → Keep the required local layout, then batch/compact Parquet objects in production.
- [OpenWeather can add fields or vary optional structures] → Preserve complete raw JSON/JSONB, model optional fields as nullable, test representative variants, and evolve the processed schema deliberately.
- [The local filesystem is not a distributed multi-writer store] → Run one local worker and use conditional object creation plus distributed coordination/object storage in production.
- [Hard termination can leave a temporary file] → Use recognizable same-directory temporary names, never publish them as final objects, and ignore/clean stale temporary files on startup.
- [API rate limits or transient failures lose a polling point because the local service does not retry] → End that run visibly, let the next tick start a new independent poll, and document bounded exponential retries and rate-limit handling only as a production enhancement.
- [A dependency-aware health endpoint is unsuitable as a pure liveness probe] → Document it as readiness/health and use process/container status for liveness in production.
- [Bind-mounted directories may not be writable by the image's non-root UID] → Create image directories with correct ownership and document host directory permissions or named-volume use.

## Migration Plan

1. Add the module, migration, implementation, tests, container assets, and documentation without changing existing data.
2. Run the full validation suite and a compose smoke test with a non-production API key/location.
3. Inspect PostgreSQL JSONB plus raw JSON and processed Parquet for one event, then verify duplicate and restart behavior.
4. Deploy a single service instance with persistent mounts and monitor logs, health, and metrics before enabling longer operation.
5. Roll back by stopping the service or restoring the prior image. Generated database rows and append-only files remain compatible evidence and need not be deleted; the new table can be dropped separately only if explicitly desired.
