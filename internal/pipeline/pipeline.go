package pipeline

import (
	"context"
	"fmt"
	"log"

	"github.com/jiminhsieh/data-engineer-golang/internal/lake"
	"github.com/jiminhsieh/data-engineer-golang/internal/observability"
	"github.com/jiminhsieh/data-engineer-golang/internal/transform"
	"github.com/jiminhsieh/data-engineer-golang/internal/weather"
)

type Fetcher interface {
	Fetch(context.Context) (weather.Result, error)
}
type Repository interface {
	Insert(context.Context, int64, float64, float64, []byte) (bool, error)
}
type Lake interface {
	WriteRaw(lake.Identity, []byte) (bool, error)
	WriteProcessed(lake.Identity, transform.Record) (bool, error)
}
type Transform func(weather.Current) (transform.Record, error)
type Pipeline struct {
	fetch     Fetcher
	repo      Repository
	lake      Lake
	transform Transform
	log       *log.Logger
	metrics   *observability.Metrics
}

func New(fetch Fetcher, repo Repository, store Lake, fn Transform, logger *log.Logger, metrics *observability.Metrics) *Pipeline {
	return &Pipeline{fetch: fetch, repo: repo, lake: store, transform: fn, log: logger, metrics: metrics}
}
func (p *Pipeline) Run(ctx context.Context) error {
	p.metrics.APITotal.Inc()
	result, err := p.fetch.Fetch(ctx)
	if err != nil {
		p.metrics.APIFailure.Inc()
		p.log.Print("stage=api outcome=failure error=\"request failed\"")
		return fmt.Errorf("api stage: %w", err)
	}
	p.metrics.APISuccess.Inc()
	id := lake.Identity{SourceDT: result.Weather.DT, Latitude: result.Weather.Coord.Latitude, Longitude: result.Weather.Coord.Longitude}
	p.log.Printf("stage=api outcome=success source_dt=%d", id.SourceDT)
	created, err := p.repo.Insert(ctx, id.SourceDT, id.Latitude, id.Longitude, result.Raw)
	if err != nil {
		p.log.Printf("stage=postgres outcome=failure source_dt=%d error=\"write failed\"", id.SourceDT)
		return fmt.Errorf("postgres stage: %w", err)
	}
	if created {
		p.metrics.DataSaved.WithLabelValues("postgres").Inc()
		p.log.Printf("stage=postgres outcome=success source_dt=%d", id.SourceDT)
	} else {
		p.log.Printf("stage=postgres outcome=duplicate source_dt=%d", id.SourceDT)
	}
	created, err = p.lake.WriteRaw(id, result.Raw)
	if err != nil {
		p.log.Printf("stage=raw outcome=failure source_dt=%d error=\"write failed\"", id.SourceDT)
		return fmt.Errorf("raw stage: %w", err)
	}
	if created {
		p.metrics.DataSaved.WithLabelValues("raw").Inc()
		p.log.Printf("stage=raw outcome=success source_dt=%d", id.SourceDT)
	} else {
		p.log.Printf("stage=raw outcome=duplicate source_dt=%d", id.SourceDT)
	}
	p.metrics.TransformTotal.Inc()
	record, err := p.transform(result.Weather)
	if err != nil {
		p.metrics.TransformErrors.Inc()
		p.log.Printf("stage=transform outcome=failure source_dt=%d error=\"transform failed\"", id.SourceDT)
		return fmt.Errorf("transform stage: %w", err)
	}
	p.metrics.TransformSuccess.Inc()
	p.log.Printf("stage=transform outcome=success source_dt=%d", id.SourceDT)
	created, err = p.lake.WriteProcessed(id, record)
	if err != nil {
		p.log.Printf("stage=processed outcome=failure source_dt=%d error=\"write failed\"", id.SourceDT)
		return fmt.Errorf("processed stage: %w", err)
	}
	if created {
		p.metrics.DataSaved.WithLabelValues("processed").Inc()
		p.log.Printf("stage=processed outcome=success source_dt=%d", id.SourceDT)
	} else {
		p.log.Printf("stage=processed outcome=duplicate source_dt=%d", id.SourceDT)
	}
	return nil
}
