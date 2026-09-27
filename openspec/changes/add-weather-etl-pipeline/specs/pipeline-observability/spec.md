# Spec Delta

## Purpose

Defines operator-visible logs, health status, and metrics needed to monitor and explain behavior of the continuously running ETL service.

## ADDED Requirements

### Requirement: Persistent ETL logging
The service SHALL use Go's standard logging facilities to append timestamped ETL messages to `<log-path>`, which defaults to `logs/etl.log`, creating parent directories and the file when needed. Logs SHALL identify API request success or failure, transformation failure, PostgreSQL and file-storage failure, successful PostgreSQL persistence, and successful raw and processed saves without including the API key or database credentials.

#### Scenario: Successful pipeline run is logged
- **WHEN** a run completes ingestion, transformation, and both file writes
- **THEN** `logs/etl.log` contains timestamped success entries identifying the request and each persistence destination

#### Scenario: Pipeline stage fails
- **WHEN** an API, transformation, database, or file operation fails
- **THEN** `logs/etl.log` contains a timestamped stage-specific error with enough context to diagnose the event and no secret values

#### Scenario: Service restarts
- **WHEN** the service opens a log path that already contains messages
- **THEN** new messages are appended and previous log content remains unchanged

### Requirement: Prometheus metrics endpoint
The service SHALL expose `GET /metrics` in Prometheus text format. Metrics SHALL report API request outcomes, pipeline run outcomes and duration, transformation errors, successful records by persistence destination, storage errors by destination, and the Unix timestamp of the last fully successful run. Metric labels SHALL use bounded enumerations and MUST NOT contain API keys, full URLs, arbitrary errors, or raw coordinates.

#### Scenario: Metrics are scraped
- **WHEN** a client sends `GET /metrics`
- **THEN** it receives a successful Prometheus-compatible response containing counters for request and run outcomes, transformation and storage behavior, a run-duration measurement, and last-success time

#### Scenario: A run fails
- **WHEN** an ETL run fails at a monitored stage
- **THEN** the matching bounded failure metric increments while the last-success timestamp remains unchanged

#### Scenario: A run succeeds
- **WHEN** PostgreSQL, raw JSON, and processed Parquet persistence all complete or are confirmed as idempotent duplicates
- **THEN** success counters are updated and last-success time is set to the completion time

### Requirement: Dependency-aware health endpoint
The service SHALL expose `GET /healthz` and test PostgreSQL connectivity using a bounded timeout. It SHALL return HTTP 200 with a JSON `ok` status when the process is serving and PostgreSQL responds, and HTTP 503 with a JSON `unhealthy` status when PostgreSQL does not respond. The response MUST NOT reveal credentials.

#### Scenario: Service and database are healthy
- **WHEN** a client requests `/healthz` while PostgreSQL responds within the health timeout
- **THEN** the service returns HTTP 200 and a JSON body with status `ok`

#### Scenario: Database is unavailable
- **WHEN** a client requests `/healthz` while PostgreSQL is unavailable or times out
- **THEN** the service returns HTTP 503 and a JSON body with status `unhealthy`

#### Scenario: Unsupported HTTP method
- **WHEN** a client uses a method other than GET on `/healthz` or `/metrics`
- **THEN** the service returns HTTP 405
