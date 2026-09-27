# Spec Delta

## Purpose

Defines reproducible local and container operation, persistent runtime storage, and documentation needed to evaluate and productionize the pipeline.

## ADDED Requirements

### Requirement: Reproducible Go validation
The repository SHALL define a Go module and documented commands that format, vet, test, and build the service. Automated tests SHALL cover configuration validation, API success and failure handling, transformation and optional fields, UTC partition and file naming, append-only duplicate behavior, PostgreSQL persistence, logs, health responses, and metrics updates.

#### Scenario: Validation runs in a clean checkout
- **WHEN** a developer with the documented Go version and required test services follows the validation instructions
- **THEN** dependencies resolve and formatting checks, vetting, tests, and build complete without relying on undeclared local files

### Requirement: Runnable application image
The repository SHALL provide a multi-stage Dockerfile that builds the service and produces a minimal runtime image containing the executable and required runtime certificates. The container SHALL run as a non-root user, accept all runtime configuration through environment variables, listen on the configured HTTP address, and write only beneath mounted data and log locations.

#### Scenario: Image starts with valid configuration
- **WHEN** the image is run with valid OpenWeather and PostgreSQL settings and writable data and log mounts
- **THEN** the service starts ingestion and serves health and metrics on the published HTTP port

#### Scenario: Image starts without required configuration
- **WHEN** the image is run without an API key or PostgreSQL connection string
- **THEN** it exits non-zero with a descriptive, secret-free configuration error

### Requirement: Persistent local orchestration
The repository SHALL provide a local container orchestration definition for the ETL service and PostgreSQL, including a PostgreSQL health check, service dependency handling, and named or bind-mounted persistence for database state, raw data, processed data, and logs. ETL configuration SHALL be supplied without committing credentials.

#### Scenario: Local stack is launched
- **WHEN** an operator provides the documented environment settings and starts the orchestration stack
- **THEN** PostgreSQL becomes healthy before the ETL service starts and generated database, lake, and log data survive container replacement

#### Scenario: ETL container is recreated
- **WHEN** the ETL container is removed and recreated with the same persistent mounts
- **THEN** prior raw, processed, and log files remain present and new output does not overwrite them

### Requirement: Operator and production documentation
The README SHALL document prerequisites, configuration variables and defaults, direct Go execution, Docker build and run commands, volume mounts, local orchestration, test commands, output schemas and paths, HTTP endpoints, metric meanings, troubleshooting, and design decisions. It SHALL explicitly state that the local implementation performs no automatic retries. It SHALL also describe a production architecture with cloud service mappings and explain scaling, scheduling, idempotency, a production retry strategy, security, observability, schema evolution, data quality, retention, backup, high availability, and disaster recovery.

#### Scenario: New operator follows the README
- **WHEN** a new operator has an OpenWeather API key and follows the README from a clean checkout
- **THEN** they can configure and run the stack, locate persisted outputs and logs, query health and metrics, and execute the test suite

#### Scenario: Reviewer assesses production readiness
- **WHEN** a reviewer reads the productionization section
- **THEN** they can identify proposed cloud components and the approach to scalability and reliability, including known differences from the local take-home implementation
