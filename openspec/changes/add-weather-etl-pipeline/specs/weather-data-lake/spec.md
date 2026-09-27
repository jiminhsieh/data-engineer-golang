# Spec Delta

## Purpose

Defines deterministic raw and processed weather lake objects that preserve source data while supporting efficient analytical reads.

## ADDED Requirements

### Requirement: Raw JSON export
For every accepted payload committed to PostgreSQL, the pipeline SHALL write the original complete JSON response to the raw zone as a `.json` object. Raw serialization SHALL preserve all source fields and values and SHALL be isolated behind a format boundary so a future raw encoder can be introduced without changing ingestion or path behavior.

#### Scenario: Raw export succeeds
- **WHEN** a new accepted payload reaches data-lake processing
- **THEN** the raw zone contains a valid JSON object with every field and value from the API response

#### Scenario: Raw export fails
- **WHEN** the raw object cannot be created or fully written
- **THEN** the run fails without automatically retrying or writing a processed object, and any incomplete raw object is removed

### Requirement: Weather transformation
The pipeline SHALL produce one normalized record from each raw response. It SHALL replace the top-level `coord` object with `coord_lon` and `coord_lat` fields, flatten every member of the top-level `main` object to a field prefixed with `main_`, flatten every member of the top-level `wind` object to a field prefixed with `wind_`, and replace top-level `dt` with `event_time`. `event_time` SHALL be the source Unix-seconds value converted to an ISO 8601 UTC string in RFC 3339 form, such as `2026-01-01T00:00:00Z`. The transformation SHALL retain all other source fields and preserve absent optional values as nullable values rather than inventing measurements.

#### Scenario: Complete payload transformation
- **WHEN** a payload contains `coord`, `main`, `wind`, `dt`, and the other documented current-weather fields
- **THEN** the normalized record has the prefixed scalar columns and an ISO 8601 UTC `event_time`, has no top-level `coord`, `main`, `wind`, or `dt` field, and retains every other source field

#### Scenario: Optional weather values are absent
- **WHEN** a valid payload omits optional values such as wind gust, rain, snow, sea-level pressure, or ground-level pressure
- **THEN** transformation succeeds with the corresponding processed values represented as null rather than invented measurements

#### Scenario: Required value is invalid
- **WHEN** source `dt`, coordinates, or a value required by the processed schema has an incompatible type or range
- **THEN** transformation fails and no processed object is written

### Requirement: Processed Parquet export
The pipeline SHALL encode each successfully transformed record only as a `.parquet` object in the processed zone. The Parquet schema SHALL store `event_time` as a UTF-8 ISO 8601 UTC value, use typed scalar columns for the flattened coordinate, main, wind, and identity values, and use typed nullable or nested columns for optional and retained structures.

#### Scenario: Processed export succeeds
- **WHEN** raw export and transformation both succeed
- **THEN** exactly one readable Parquet record representing that weather event is available in the processed object

#### Scenario: Processed export fails
- **WHEN** Parquet encoding or storage fails
- **THEN** the run fails without automatically retrying and any incomplete processed object is removed while the committed PostgreSQL row and completed raw object remain available

### Requirement: Event-date layout and deterministic names
Raw and processed objects SHALL be stored beneath `<data-root>/raw/dt=YYYYMMDD/` and `<data-root>/processed/dt=YYYYMMDD/`, respectively. `YYYYMMDD` SHALL be derived in UTC from the payload's numeric top-level `dt` value. Each object's base name SHALL be `<source_dt>_<lat>_<lon>`, using the unconverted source `dt` and stable decimal representations of response latitude and longitude, followed by `.json` in the raw zone or `.parquet` in the processed zone.

#### Scenario: Event is assigned to a partition
- **WHEN** a payload has source `dt` value `1767225600`, corresponding to 2026-01-01 in UTC, and coordinates latitude 44.34 and longitude 10.99
- **THEN** its paths are `raw/dt=20260101/1767225600_44.34_10.99.json` and `processed/dt=20260101/1767225600_44.34_10.99.parquet`

#### Scenario: UTC date differs from local date
- **WHEN** the configured location's local date differs from the UTC date of the event
- **THEN** the pipeline uses the UTC date for both partitions

### Requirement: Append-only and idempotent files
The pipeline MUST NOT overwrite an existing raw or processed object. A distinct source `dt`, latitude, and longitude identity SHALL create a distinct object. Reprocessing an event whose correctly named object already exists SHALL treat that object as an idempotent duplicate and leave it unchanged; other storage conflicts SHALL fail visibly.

#### Scenario: Distinct event is processed
- **WHEN** no object exists at the deterministic raw or processed path
- **THEN** the pipeline creates a new object without changing earlier event objects

#### Scenario: Event is reprocessed
- **WHEN** the deterministic object for the same event identity already exists
- **THEN** the existing object remains byte-for-byte unchanged and is not appended to or replaced
