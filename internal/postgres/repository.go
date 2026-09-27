package postgres

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/001_weather_raw.sql
var migration string

type DB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Ping(context.Context) error
}

type Repository struct{ db DB }

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, errors.New("parse PostgreSQL configuration")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("connect to PostgreSQL")
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.New("ping PostgreSQL")
	}
	return pool, nil
}

func NewRepository(db DB) *Repository { return &Repository{db: db} }
func (r *Repository) Migrate(ctx context.Context) error {
	if _, err := r.db.Exec(ctx, migration); err != nil {
		return fmt.Errorf("apply PostgreSQL migration: %w", err)
	}
	return nil
}
func (r *Repository) Ping(ctx context.Context) error { return r.db.Ping(ctx) }
func (r *Repository) Insert(ctx context.Context, sourceDT int64, latitude, longitude float64, raw []byte) (bool, error) {
	tag, err := r.db.Exec(ctx, `INSERT INTO weather_raw (source_dt, latitude, longitude, payload) VALUES ($1, $2, $3, $4::jsonb) ON CONFLICT (source_dt, latitude, longitude) DO NOTHING`, sourceDT, latitude, longitude, raw)
	if err != nil {
		return false, fmt.Errorf("insert weather event: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}
