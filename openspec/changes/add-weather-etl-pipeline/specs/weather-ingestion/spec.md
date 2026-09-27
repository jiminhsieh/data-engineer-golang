# Spec Delta

## Purpose

Defines reliable scheduled collection of current weather payloads and durable raw persistence in PostgreSQL before downstream processing.

## ADDED Requirements

### Requirement: Runtime configuration
The service SHALL read its OpenWeather API key, target latitude and longitude, PostgreSQL connection string, HTTP listen address, data root, log path, request timeout, and polling interval from runtime configuration. The polling interval SHALL default to 30 seconds. The service MUST reject a missing API key, invalid coordinates, a non-positive duration, or an unusable PostgreSQL connection before starting scheduled ingestion.

#### Scenario: Valid configuration
- **WHEN** all required values are provided and optional values are omitted
- **THEN** the service starts with a 30-second polling interval and documented defaults for the other optional settings

#### Scenario: Invalid required configuration
- **WHEN** the API key is blank, latitude or longitude is outside its valid range, a duration is non-positive, or PostgreSQL is unavailable during startup
- **THEN** the service exits with a descriptive error without issuing a weather request

### Requirement: Scheduled OpenWeather extraction
The service SHALL request current weather for the configured coordinates once at startup and subsequently every configured interval. Requests SHALL use the OpenWeather current-weather endpoint over HTTP(S), include the configured API key and coordinates, apply a finite timeout, and execute serially so slow runs do not overlap. Each scheduled run SHALL make exactly one API request attempt and MUST NOT automatically retry a failed request.

#### Scenario: Successful scheduled request
- **WHEN** OpenWeather returns a successful HTTP response containing a valid current-weather payload
- **THEN** the service accepts that payload for persistence and downstream processing

#### Scenario: Remote or payload failure
- **WHEN** a request times out, cannot connect, returns a non-success status, contains invalid JSON, or lacks valid source `dt` or coordinate fields
- **THEN** the current run fails without retrying, persisting that response, or invoking downstream file processing, and the next independently scheduled run remains enabled

### Requirement: Raw PostgreSQL persistence
For every accepted response, the service SHALL persist the complete raw JSON value in a PostgreSQL JSONB column together with its source `dt` Unix value, response coordinates, and ingestion time. The service SHALL maintain the required table automatically and SHALL treat source `dt` plus latitude and longitude as the logical event identity.

#### Scenario: New weather event
- **WHEN** an accepted payload has an event identity not present in PostgreSQL
- **THEN** one row containing the complete response as JSONB and its metadata is committed before data-lake processing begins

#### Scenario: Repeated weather event
- **WHEN** an accepted payload has the same source `dt` and coordinates as an existing row
- **THEN** no additional logical row is created and downstream outputs are still checked so a prior partial run can recover

#### Scenario: Database write failure
- **WHEN** PostgreSQL cannot commit an accepted payload
- **THEN** the run fails without automatically retrying, no data-lake output is attempted for that run, and the scheduler continues with the next interval

### Requirement: Graceful service lifecycle
The HTTP endpoints and ingestion scheduler SHALL run concurrently until the process receives a termination signal. Shutdown SHALL stop new scheduled runs, allow in-flight work a bounded opportunity to finish, stop the HTTP server, close PostgreSQL resources, and close the log file.

#### Scenario: Termination signal
- **WHEN** the running service receives SIGTERM or SIGINT
- **THEN** it performs bounded graceful shutdown and exits without starting another ingestion run
