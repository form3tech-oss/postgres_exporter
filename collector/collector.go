// Copyright 2022 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collector

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	factories              = make(map[string]func(collectorConfig) (Collector, error))
	initiatedCollectorsMtx = sync.Mutex{}
	initiatedCollectors    = make(map[string]Collector)
	collectorState         = make(map[string]*bool)
	forcedCollectors       = map[string]bool{} // collectors which have been explicitly enabled or disabled
)

const (
	// Namespace for all metrics.
	namespace = "pg"

	defaultEnabled  = true
	defaultDisabled = false

	defaultMaxOpenConnections = 10
	defaultIdleConnections    = 5
)

var (
	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_duration_seconds"),
		"postgres_exporter: Duration of a collector scrape.",
		[]string{"collector"},
		nil,
	)
	scrapeSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_success"),
		"postgres_exporter: Whether a collector succeeded.",
		[]string{"collector"},
		nil,
	)
)

type Collector interface {
	Update(ctx context.Context, instance *instance, ch chan<- prometheus.Metric) error
}

type collectorConfig struct {
	logger           log.Logger
	excludeDatabases []string
	constantLabels   prometheus.Labels
}

func registerCollector(name string, isDefaultEnabled bool, createFunc func(collectorConfig) (Collector, error)) {
	var helpDefaultState string
	if isDefaultEnabled {
		helpDefaultState = "enabled"
	} else {
		helpDefaultState = "disabled"
	}

	// Create flag for this collector
	flagName := fmt.Sprintf("collector.%s", name)
	flagHelp := fmt.Sprintf("Enable the %s collector (default: %s).", name, helpDefaultState)
	defaultValue := fmt.Sprintf("%v", isDefaultEnabled)

	flag := kingpin.Flag(flagName, flagHelp).Default(defaultValue).Action(collectorFlagAction(name)).Bool()
	collectorState[name] = flag

	// Register the create function for this collector
	factories[name] = createFunc
}

// PostgresCollector implements the prometheus.Collector interface.
type PostgresCollector struct {
	Collectors map[string]Collector
	logger     log.Logger

	instance           *instance
	constantLabels     prometheus.Labels
	maxOpenConnections int
	maxIdleConnections int
	scrapeTimeout      time.Duration
}

type Option func(*PostgresCollector) error

// WithConstantLabels configures constant labels.
func WithConstantLabels(l prometheus.Labels) Option {
	return func(c *PostgresCollector) error {
		c.constantLabels = l
		return nil
	}
}

// WithMaxOpenConnections configures the max number of open connections kept in the underlying pool.
func WithMaxOpenConnections(v int) Option {
	return func(c *PostgresCollector) error {
		c.maxOpenConnections = v
		return nil
	}
}

// WithMaxIdleConnections configures the max number of idle connections kept in the underlying pool.
func WithMaxIdleConnections(v int) Option {
	return func(c *PostgresCollector) error {
		c.maxIdleConnections = v
		return nil
	}
}

// WithScrapeTimeout configures the timeout for a single collector scrape.
func WithScrapeTimeout(t time.Duration) Option {
	return func(c *PostgresCollector) error {
		c.scrapeTimeout = t
		return nil
	}
}

// NewPostgresCollector creates a new PostgresCollector.
func NewPostgresCollector(logger log.Logger, excludeDatabases []string, dsn string, filters []string, options ...Option) (*PostgresCollector, error) {
	p := &PostgresCollector{
		logger:             logger,
		scrapeTimeout:      5 * time.Second,
		maxOpenConnections: defaultMaxOpenConnections,
		maxIdleConnections: defaultIdleConnections,
	}
	// Apply options to customize the collector
	for _, o := range options {
		err := o(p)
		if err != nil {
			return nil, err
		}
	}

	f := make(map[string]bool)
	for _, filter := range filters {
		enabled, exist := collectorState[filter]
		if !exist {
			return nil, fmt.Errorf("missing collector: %s", filter)
		}
		if !*enabled {
			return nil, fmt.Errorf("disabled collector: %s", filter)
		}
		f[filter] = true
	}
	collectors := make(map[string]Collector)
	initiatedCollectorsMtx.Lock()
	defer initiatedCollectorsMtx.Unlock()
	for key, enabled := range collectorState {
		level.Debug(logger).Log("msg", "collector state", "name", key, "enabled", enabled)
		if !*enabled || (len(f) > 0 && !f[key]) {
			continue
		}
		if collector, ok := initiatedCollectors[key]; ok {
			collectors[key] = collector
		} else {
			collector, err := factories[key](collectorConfig{
				logger:           log.With(logger, "collector", key),
				excludeDatabases: excludeDatabases,
				constantLabels:   p.constantLabels,
			})
			if err != nil {
				return nil, err
			}
			collectors[key] = collector
			initiatedCollectors[key] = collector
		}
	}

	p.Collectors = collectors

	if dsn == "" {
		return nil, errors.New("empty dsn")
	}

	instanceConf := &instanceConfiguration{
		dbMaxOpenConns: p.maxOpenConnections,
		dbMaxIdleConns: p.maxIdleConnections,
	}
	instance, err := newInstance(dsn, instanceConf)
	if err != nil {
		return nil, err
	}

	err = instance.setup()
	if err != nil {
		level.Error(p.logger).Log("msg", "setting up connection to database", "err", err)
		return nil, err
	}
	p.instance = instance

	return p, nil
}

// Close closes the underlying collector instance
func (p PostgresCollector) Close() error {
	level.Debug(p.logger).Log("msg", "closing collector", "instance", p.instance)
	return p.instance.Close()
}

// Describe implements the prometheus.Collector interface.
func (p PostgresCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
}

// Collect implements the prometheus.Collector interface.
func (p PostgresCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	wg := sync.WaitGroup{}
	wg.Add(len(p.Collectors))
	for name, c := range p.Collectors {
		go func(name string, c Collector) {
			execute(ctx, p.scrapeTimeout, name, c, p.instance, ch, p.logger, &wg)
		}(name, c)
	}
	wg.Wait()
}

func execute(ctx context.Context, timeout time.Duration, name string, c Collector, instance *instance, ch chan<- prometheus.Metric, logger log.Logger, wg *sync.WaitGroup) {
	defer wg.Done()
	begin := time.Now()

	scrapeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	err := c.Update(scrapeCtx, instance, ch)
	duration := time.Since(begin)
	var success float64

	if err != nil {
		success = 0
		if IsNoDataError(err) {
			level.Debug(logger).Log("msg", "collector returned no data", "name", name, "duration_seconds", duration.Seconds(), "err", err)
		} else if scrapeCtx.Err() == context.DeadlineExceeded {
			level.Error(logger).Log("msg", "collector timedout", "name", name, "duration_seconds", duration.Seconds(), "err", err)
		} else {
			level.Error(logger).Log("msg", "collector failed", "name", name, "duration_seconds", duration.Seconds(), "err", err)
		}
	} else {
		level.Info(logger).Log("msg", "collector succeeded", "name", name, "duration_seconds", duration.Seconds())
		success = 1
	}
	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, duration.Seconds(), name)
	ch <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, success, name)
}

// collectorFlagAction generates a new action function for the given collector
// to track whether it has been explicitly enabled or disabled from the command line.
// A new action function is needed for each collector flag because the ParseContext
// does not contain information about which flag called the action.
// See: https://github.com/alecthomas/kingpin/issues/294
func collectorFlagAction(collector string) func(ctx *kingpin.ParseContext) error {
	return func(ctx *kingpin.ParseContext) error {
		forcedCollectors[collector] = true
		return nil
	}
}

// ErrNoData indicates the collector found no data to collect, but had no other error.
var ErrNoData = errors.New("collector returned no data")

func IsNoDataError(err error) bool {
	return err == ErrNoData
}
