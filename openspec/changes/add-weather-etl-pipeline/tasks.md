# Tasks

## 1. Project Foundation and Configuration

- [ ] 1.1 Initialize the Go module and `cmd/weather-etl` plus focused `internal` package layout, add only the PostgreSQL, Prometheus, and Parquet dependencies selected in the design, and verify `go mod tidy` and `go list ./...` succeed.
- [ ] 1.2 Implement environment parsing, defaults, validation, and secret-safe errors for API, coordinates, durations, addresses, paths, and PostgreSQL settings; verify table-driven configuration tests cover valid defaults and every invalid class in the ingestion spec.
- [ ] 1.3 Define typed OpenWeather input and processed-record models with nullable optional fields plus representative success/error fixtures; verify decoding tests cover complete, optional, malformed, and missing-identity payloads.
- [ ] 1.4 Add safe generated-data/log exclusions and a credential-free `.env.example`, then document prerequisites and every configuration variable/default in README and verify the sample environment is accepted after placeholder secrets are supplied.

## 2. Extraction and PostgreSQL Durability

- [ ] 2.1 Implement the bounded, single-attempt OpenWeather current-weather client with coordinate/API-key query construction, response-size protection, status checking, and raw-byte preservation; verify `httptest` cases cover success, timeout, non-2xx, oversized, malformed, and invalid-identity responses, assert exactly one request per invocation, and confirm errors do not expose the key.
- [ ] 2.2 Add and embed the `weather_raw` migration, connect/ping logic, and idempotent JSONB repository insert keyed by numeric source `dt` and coordinates; verify the PostgreSQL integration test applies the migration, retains the complete JSON value, stores UTC ingestion metadata, makes one statement attempt on failure, and leaves one row after a duplicate insert.
- [ ] 2.3 Document the extraction flow, PostgreSQL schema, source-`dt` event identity, absence of automatic retries, database test setup, and stage-failure behavior in README; verify all shown SQL and integration-test commands run against the compose PostgreSQL service.

## 3. Transformation and Data-Lake Storage

- [ ] 3.1 Implement the typed transformation that flattens `coord`, `main`, and `wind`, converts source `dt` to an RFC 3339 ISO 8601 UTC `event_time` string, and retains every other source field; verify table-driven tests cover the exact UTC conversion, a full payload, all nullable optionals, retained fields, and incompatible required values.
- [ ] 3.2 Implement UTC daily path construction from numeric source `dt`, stable coordinate formatting, the raw-encoder boundary, and exact JSON writes using temporary/no-replace publication; verify temporary-directory tests assert `raw/dt=20260101/1767225600_44.34_10.99.json`, exact source content, cleanup without retries on error, and byte-for-byte preservation on duplicate output.
- [ ] 3.3 Implement Parquet as the only processed format, writing one record with UTF-8 ISO 8601 `event_time`, typed flattened columns, nullable/nested retained columns, and no-replace behavior; verify a read-back test checks timestamp text, flattened values, retained structures, optional nulls, one-record cardinality, single-attempt error cleanup, and unchanged duplicate output.
- [ ] 3.4 Document the raw JSON and processed Parquet schemas, ISO 8601 `event_time`, source-`dt` filenames, sample paths, daily UTC partition rationale, duplicate semantics, and raw JSON-to-future-Avro boundary in README; verify the examples agree with fixture-generated paths and Parquet columns.

## 4. Pipeline, Logging, Metrics, and HTTP Endpoints

- [ ] 4.1 Implement append-only `logs/etl.log` initialization and stage-specific standard-library log messages, optionally mirrored to stdout; verify tests reopen the logger without truncation and confirm success/failure messages include useful context but no API key, DSN, or credential-bearing URL.
- [ ] 4.2 Implement an isolated Prometheus registry with the bounded counters, histogram, and last-success gauge from the design; verify metric-gathering tests exercise success, each stage failure, and duplicate outcomes with no coordinate or arbitrary-error labels.
- [ ] 4.3 Implement GET-only `/metrics` and dependency-aware `/healthz` handlers with bounded database pings and minimal JSON; verify handler tests cover Prometheus output, healthy 200, unhealthy 503, credential-free bodies, and 405 responses.
- [ ] 4.4 Implement the serial pipeline in fetch → PostgreSQL → raw → transform → Parquet order, including logging, metrics, idempotent continuation, no automatic retries, and stage short-circuiting; verify fake-boundary tests assert call order, one attempt per stage, every failure branch, partial-output behavior, duplicate repair, and last-success updates.
- [ ] 4.5 Implement immediate-then-interval scheduling and context cancellation without overlapping runs; verify deterministic scheduler tests demonstrate immediate execution, serial slow runs, no same-run retry after error, a later independent scheduled run, and no new run after cancellation.
- [ ] 4.6 Document log location/events, endpoint requests and responses, every metric and label, health semantics, scheduler behavior, no-retry failure handling, and troubleshooting in README; verify the documented curl commands match HTTP tests and exposed metric names.

## 5. Runtime Assembly and Container Operation

- [ ] 5.1 Compose configuration, logger, migrations/repository, client, writers, pipeline, scheduler, and HTTP server in `cmd/weather-etl`, with bounded SIGINT/SIGTERM shutdown and resource closure; verify build plus lifecycle tests show startup failure on invalid dependencies and graceful cancellation of a running service.
- [ ] 5.2 Add a multi-stage Dockerfile and `.dockerignore` with pinned Go tooling, CA certificates, a non-root runtime user, writable application directories, and no embedded generated data or secrets; verify `docker build` succeeds and image inspection confirms the configured non-root user.
- [ ] 5.3 Add `compose.yaml` for healthy PostgreSQL startup, ETL dependency ordering, environment injection, published monitoring port, and persistent database/data/log mounts; verify `docker compose config` succeeds with the example environment and contains health checks and all required mounts.
- [ ] 5.4 Complete README direct-Go, Docker, compose, volume-permission, test, output-inspection, restart, and rollback instructions; verify a reviewer can copy each command from a clean checkout without relying on undeclared files.
- [ ] 5.5 Complete README design-rationale and productionization sections covering AWS service mappings, decoupled scaling, ownership/scheduling, idempotency, production-only bounded retries and DLQ replay, schema/data quality, security, observability, compaction/retention, backup, multi-AZ availability, and disaster recovery; verify each production concern required by the containerized-operation spec is explicitly addressed without implying retries exist locally.

## 6. End-to-End Verification

- [ ] 6.1 Run formatting checks, `go vet ./...`, unit tests, the PostgreSQL integration suite, and `go build ./...`; fix all failures and verify the full validation sequence exits successfully from a clean checkout.
- [ ] 6.2 Run the compose stack with a non-production OpenWeather key, verify `/healthz` and `/metrics`, inspect one JSONB row plus matching raw JSON and readable processed Parquet, confirm Parquet contains ISO 8601 UTC `event_time`, and verify logs record every successful stage.
- [ ] 6.3 Reprocess or wait for a duplicate event and recreate the ETL container; verify row/file counts do not duplicate the logical event, existing file hashes do not change, and database, lake, and log state survives recreation.
- [ ] 6.4 Run `openspec validate add-weather-etl-pipeline --strict` and compare delivered behavior and README coverage against all four capability specs; verify validation passes with no unaddressed requirement or scenario.
