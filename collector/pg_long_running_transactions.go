// Copyright 2023 The Prometheus Authors
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

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

const longRunningTransactionsSubsystem = "long_running_transactions"

func init() {
	registerCollector(longRunningTransactionsSubsystem, defaultDisabled, NewPGLongRunningTransactionsCollector)
}

type PGLongRunningTransactionsCollector struct {
	log log.Logger
	longRunningTransactionsCount *prometheus.Desc
	longRunningTransactionsAgeInSeconds *prometheus.Desc
}

func NewPGLongRunningTransactionsCollector(config collectorConfig) (Collector, error) {
	return &PGLongRunningTransactionsCollector{
		log: config.logger,
		longRunningTransactionsCount : prometheus.NewDesc(
			"pg_long_running_transactions",
			"Current number of long running transactions",
			[]string{},
			config.constantLabels,
		),
		longRunningTransactionsAgeInSeconds : prometheus.NewDesc(
			prometheus.BuildFQName(namespace, longRunningTransactionsSubsystem, "oldest_timestamp_seconds"),
			"The current maximum transaction age in seconds",
			[]string{},
			config.constantLabels,
		),
	}, nil
}

var (
	longRunningTransactionsQuery = `
	SELECT
		COUNT(*) as transactions,
   		MAX(EXTRACT(EPOCH FROM clock_timestamp())) AS oldest_timestamp_seconds
    FROM pg_catalog.pg_stat_activity
    WHERE state is distinct from 'idle' AND query not like 'autovacuum:%'
	`
)

func (c *PGLongRunningTransactionsCollector) Update(ctx context.Context, instance *instance, ch chan<- prometheus.Metric) error {
	db := instance.getDB()
	rows, err := db.QueryContext(ctx,
		longRunningTransactionsQuery)

	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var transactions, ageInSeconds float64

		if err := rows.Scan(&transactions, &ageInSeconds); err != nil {
			return err
		}

		ch <- prometheus.MustNewConstMetric(
			c.longRunningTransactionsCount,
			prometheus.GaugeValue,
			transactions,
		)
		ch <- prometheus.MustNewConstMetric(
			c.longRunningTransactionsAgeInSeconds,
			prometheus.GaugeValue,
			ageInSeconds,
		)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}
