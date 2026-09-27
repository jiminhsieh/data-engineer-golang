package observability

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Log struct {
	*log.Logger
	file *os.File
}

func OpenLog(path string, mirror io.Writer) (*Log, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return nil, fmt.Errorf("open ETL log: %w", err)
	}
	var out io.Writer = f
	if mirror != nil {
		out = io.MultiWriter(f, mirror)
	}
	return &Log{Logger: log.New(out, "", log.LstdFlags|log.LUTC), file: f}, nil
}
func (l *Log) Close() error { return l.file.Close() }

type Metrics struct {
	Registry                                          *prometheus.Registry
	APITotal, APISuccess, APIFailure                  prometheus.Counter
	TransformTotal, TransformSuccess, TransformErrors prometheus.Counter
	DataSaved                                         *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	m := &Metrics{Registry: prometheus.NewRegistry(), APITotal: prometheus.NewCounter(prometheus.CounterOpts{Name: "weather_api_requests_total", Help: "OpenWeather API request attempts."}), APISuccess: prometheus.NewCounter(prometheus.CounterOpts{Name: "weather_api_requests_success_total", Help: "Accepted OpenWeather responses."}), APIFailure: prometheus.NewCounter(prometheus.CounterOpts{Name: "weather_api_requests_failure_total", Help: "Failed or unacceptable OpenWeather responses."}), TransformTotal: prometheus.NewCounter(prometheus.CounterOpts{Name: "etl_transform_total", Help: "Weather transformation attempts."}), TransformSuccess: prometheus.NewCounter(prometheus.CounterOpts{Name: "etl_transform_success_total", Help: "Successful weather transformations."}), TransformErrors: prometheus.NewCounter(prometheus.CounterOpts{Name: "etl_transform_errors_total", Help: "Failed weather transformations."}), DataSaved: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "etl_data_saved_total", Help: "Newly persisted weather events."}, []string{"destination"})}
	m.Registry.MustRegister(m.APITotal, m.APISuccess, m.APIFailure, m.TransformTotal, m.TransformSuccess, m.TransformErrors, m.DataSaved)
	for _, d := range []string{"postgres", "raw", "processed"} {
		m.DataSaved.WithLabelValues(d)
	}
	return m
}

type Pinger interface{ Ping(context.Context) error }
type BoundedPinger struct {
	Pinger  Pinger
	Timeout time.Duration
}

func (b BoundedPinger) Ping(ctx context.Context) error {
	timeout := b.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return b.Pinger.Ping(ctx)
}
