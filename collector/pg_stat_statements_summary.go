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
	"database/sql"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

const statStatementsSummarySubsystem = "stat_statements_summary"

func init() {
	registerCollector(statStatementsSummarySubsystem, defaultDisabled, NewPGStatStatementsSummaryCollector)
}

type PGStatStatementsSummaryCollector struct {
	log                               log.Logger
	statStatementsSummaryCallsTotal   *prometheus.Desc
	statStatementsSummarySecondsTotal *prometheus.Desc
}

func NewPGStatStatementsSummaryCollector(config collectorConfig) (Collector, error) {
	return &PGStatStatementsSummaryCollector{
		log: config.logger,
		statStatementsSummaryCallsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statStatementsSummarySubsystem, "calls_total"),
			"Number of times executed",
			[]string{"datname"},
			config.constantLabels,
		),
		statStatementsSummarySecondsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statStatementsSummarySubsystem, "seconds_total"),
			"Total time spent in the statement, in seconds",
			[]string{"datname"},
			config.constantLabels,
		),
	}, nil
}

var (
	pgStatStatementsSummaryQuery = `SELECT
  	pg_database.datname,
  	SUM(pg_stat_statements.calls) as calls_total,
  	SUM(pg_stat_statements.total_exec_time) / 1000.0 as seconds_total
  	FROM pg_stat_statements
  JOIN pg_database
          ON pg_database.oid = pg_stat_statements.dbid
  WHERE
    total_exec_time > (
    SELECT percentile_cont(0.1)
            WITHIN GROUP (ORDER BY total_exec_time)
            FROM pg_stat_statements
    )
  GROUP BY pg_database.datname;`
)

func (c *PGStatStatementsSummaryCollector) Update(ctx context.Context, instance *instance, ch chan<- prometheus.Metric) error {
	query := pgStatStatementsSummaryQuery

	db := instance.getDB()
	rows, err := db.QueryContext(ctx, query)

	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var datname sql.NullString
		var callsTotal sql.NullInt64
		var secondsTotal sql.NullFloat64

		if err := rows.Scan(&datname, &callsTotal, &secondsTotal); err != nil {
			return err
		}

		datnameLabel := "unknown"
		if datname.Valid {
			datnameLabel = datname.String
		}

		callsTotalMetric := 0.0
		if callsTotal.Valid {
			callsTotalMetric = float64(callsTotal.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statStatementsSummaryCallsTotal,
			prometheus.CounterValue,
			callsTotalMetric,
			datnameLabel,
		)

		secondsTotalMetric := 0.0
		if secondsTotal.Valid {
			secondsTotalMetric = secondsTotal.Float64
		}
		ch <- prometheus.MustNewConstMetric(
			c.statStatementsSummarySecondsTotal,
			prometheus.CounterValue,
			secondsTotalMetric,
			datnameLabel,
		)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}
