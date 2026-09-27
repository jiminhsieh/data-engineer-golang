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
The service SHALL expose `GET /metrics` in Prometheus text format and SHALL update metrics in the same operation paths that emit required log events rather than deriving metrics from log parsing. It SHALL expose only these application metrics: `weather_api_requests_total`, `weather_api_requests_success_total`, `weather_api_requests_failure_total`, `etl_transform_total`, `etl_transform_success_total`, `etl_transform_errors_total`, and `etl_data_saved_total{destination}`. The `destination` label MUST be one of `postgres`, `raw`, or `processed`; metrics MUST NOT contain API keys, full URLs, arbitrary errors, or raw coordinates.

#### Scenario: Metrics are scraped
- **WHEN** a client sends `GET /metrics`
- **THEN** it receives a successful Prometheus-compatible response containing the API total/success/failure counters, transformation total/success/error counters, and destination-labeled successful-save counter

#### Scenario: API request succeeds
- **WHEN** one API request attempt returns an accepted payload
- **THEN** `weather_api_requests_total` and `weather_api_requests_success_total` each increment once and `weather_api_requests_failure_total` does not increment

#### Scenario: API request fails
- **WHEN** one API request attempt fails or returns an unacceptable payload
- **THEN** `weather_api_requests_total` and `weather_api_requests_failure_total` each increment once and `weather_api_requests_success_total` does not increment

#### Scenario: Transformation succeeds
- **WHEN** one transformation attempt completes successfully
- **THEN** `etl_transform_total` and `etl_transform_success_total` each increment once and `etl_transform_errors_total` does not increment

#### Scenario: Transformation fails
- **WHEN** one transformation attempt fails
- **THEN** `etl_transform_total` and `etl_transform_errors_total` each increment once and `etl_transform_success_total` does not increment

#### Scenario: Data is newly saved
- **WHEN** data is newly persisted to PostgreSQL, the raw zone, or the processed zone
- **THEN** `etl_data_saved_total` increments once for the corresponding `destination` value

#### Scenario: Existing data is encountered
- **WHEN** persistence reports an idempotent duplicate without creating new data
- **THEN** `etl_data_saved_total` does not increment for that destination

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
