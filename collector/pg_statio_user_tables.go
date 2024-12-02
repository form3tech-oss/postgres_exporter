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

const statioUserTableSubsystem = "statio_user_tables"

func init() {
	registerCollector(statioUserTableSubsystem, defaultEnabled, NewPGStatIOUserTablesCollector)
}

type PGStatIOUserTablesCollector struct {
	log                           log.Logger
	statioUserTablesHeapBlksRead  *prometheus.Desc
	statioUserTablesHeapBlksHit   *prometheus.Desc
	statioUserTablesIdxBlksRead   *prometheus.Desc
	statioUserTablesIdxBlksHit    *prometheus.Desc
	statioUserTablesToastBlksRead *prometheus.Desc
	statioUserTablesToastBlksHit  *prometheus.Desc
	statioUserTablesTidxBlksRead  *prometheus.Desc
	statioUserTablesTidxBlksHit   *prometheus.Desc
}

func NewPGStatIOUserTablesCollector(config collectorConfig) (Collector, error) {
	return &PGStatIOUserTablesCollector{
		log: config.logger,
		statioUserTablesHeapBlksRead: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statioUserTableSubsystem, "heap_blocks_read"),
			"Number of disk blocks read from this table",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statioUserTablesHeapBlksHit: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statioUserTableSubsystem, "heap_blocks_hit"),
			"Number of buffer hits in this table",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statioUserTablesIdxBlksRead: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statioUserTableSubsystem, "idx_blocks_read"),
			"Number of disk blocks read from all indexes on this table",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statioUserTablesIdxBlksHit: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statioUserTableSubsystem, "idx_blocks_hit"),
			"Number of buffer hits in all indexes on this table",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statioUserTablesToastBlksRead: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statioUserTableSubsystem, "toast_blocks_read"),
			"Number of disk blocks read from this table's TOAST table (if any)",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statioUserTablesToastBlksHit: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statioUserTableSubsystem, "toast_blocks_hit"),
			"Number of buffer hits in this table's TOAST table (if any)",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statioUserTablesTidxBlksRead: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statioUserTableSubsystem, "tidx_blocks_read"),
			"Number of disk blocks read from this table's TOAST table indexes (if any)",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statioUserTablesTidxBlksHit: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, statioUserTableSubsystem, "tidx_blocks_hit"),
			"Number of buffer hits in this table's TOAST table indexes (if any)",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
	}, nil
}

var statioUserTablesQuery = `SELECT
		current_database() datname,
		schemaname,
		relname,
		heap_blks_read,
		heap_blks_hit,
		idx_blks_read,
		idx_blks_hit,
		toast_blks_read,
		toast_blks_hit,
		tidx_blks_read,
		tidx_blks_hit
	FROM pg_statio_user_tables`

func (c *PGStatIOUserTablesCollector) Update(ctx context.Context, instance *instance, ch chan<- prometheus.Metric) error {
	db := instance.getDB()
	rows, err := db.QueryContext(ctx,
		statioUserTablesQuery)

	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var datname, schemaname, relname sql.NullString
		var heapBlksRead, heapBlksHit, idxBlksRead, idxBlksHit, toastBlksRead, toastBlksHit, tidxBlksRead, tidxBlksHit sql.NullInt64

		if err := rows.Scan(&datname, &schemaname, &relname, &heapBlksRead, &heapBlksHit, &idxBlksRead, &idxBlksHit, &toastBlksRead, &toastBlksHit, &tidxBlksRead, &tidxBlksHit); err != nil {
			return err
		}
		datnameLabel := "unknown"
		if datname.Valid {
			datnameLabel = datname.String
		}
		schemanameLabel := "unknown"
		if schemaname.Valid {
			schemanameLabel = schemaname.String
		}
		relnameLabel := "unknown"
		if relname.Valid {
			relnameLabel = relname.String
		}

		heapBlksReadMetric := 0.0
		if heapBlksRead.Valid {
			heapBlksReadMetric = float64(heapBlksRead.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statioUserTablesHeapBlksRead,
			prometheus.CounterValue,
			heapBlksReadMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		heapBlksHitMetric := 0.0
		if heapBlksHit.Valid {
			heapBlksHitMetric = float64(heapBlksHit.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statioUserTablesHeapBlksHit,
			prometheus.CounterValue,
			heapBlksHitMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		idxBlksReadMetric := 0.0
		if idxBlksRead.Valid {
			idxBlksReadMetric = float64(idxBlksRead.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statioUserTablesIdxBlksRead,
			prometheus.CounterValue,
			idxBlksReadMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		idxBlksHitMetric := 0.0
		if idxBlksHit.Valid {
			idxBlksHitMetric = float64(idxBlksHit.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statioUserTablesIdxBlksHit,
			prometheus.CounterValue,
			idxBlksHitMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		toastBlksReadMetric := 0.0
		if toastBlksRead.Valid {
			toastBlksReadMetric = float64(toastBlksRead.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statioUserTablesToastBlksRead,
			prometheus.CounterValue,
			toastBlksReadMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		toastBlksHitMetric := 0.0
		if toastBlksHit.Valid {
			toastBlksHitMetric = float64(toastBlksHit.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statioUserTablesToastBlksHit,
			prometheus.CounterValue,
			toastBlksHitMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		tidxBlksReadMetric := 0.0
		if tidxBlksRead.Valid {
			tidxBlksReadMetric = float64(tidxBlksRead.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statioUserTablesTidxBlksRead,
			prometheus.CounterValue,
			tidxBlksReadMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		tidxBlksHitMetric := 0.0
		if tidxBlksHit.Valid {
			tidxBlksHitMetric = float64(tidxBlksHit.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statioUserTablesTidxBlksHit,
			prometheus.CounterValue,
			tidxBlksHitMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)
	}
	return rows.Err()
}
