CREATE TABLE IF NOT EXISTS weather_raw (
    id BIGSERIAL PRIMARY KEY,
    source_dt BIGINT NOT NULL,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    payload JSONB NOT NULL,
    ingested_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT weather_raw_event_identity UNIQUE (source_dt, latitude, longitude)
);
