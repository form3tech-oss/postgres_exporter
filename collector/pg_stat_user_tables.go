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

const userTableSubsystem = "stat_user_tables"

func init() {
	registerCollector(userTableSubsystem, defaultEnabled, NewPGStatUserTablesCollector)
}

type PGStatUserTablesCollector struct {
	log                            log.Logger
	statUserTablesSeqScan          *prometheus.Desc
	statUserTablesSeqTupRead       *prometheus.Desc
	statUserTablesIdxScan          *prometheus.Desc
	statUserTablesIdxTupFetch      *prometheus.Desc
	statUserTablesNTupIns          *prometheus.Desc
	statUserTablesNTupUpd          *prometheus.Desc
	statUserTablesNTupDel          *prometheus.Desc
	statUserTablesNTupHotUpd       *prometheus.Desc
	statUserTablesNLiveTup         *prometheus.Desc
	statUserTablesNDeadTup         *prometheus.Desc
	statUserTablesNModSinceAnalyze *prometheus.Desc
	statUserTablesLastVacuum       *prometheus.Desc
	statUserTablesLastAutovacuum   *prometheus.Desc
	statUserTablesLastAnalyze      *prometheus.Desc
	statUserTablesLastAutoanalyze  *prometheus.Desc
	statUserTablesVacuumCount      *prometheus.Desc
	statUserTablesAutovacuumCount  *prometheus.Desc
	statUserTablesAnalyzeCount     *prometheus.Desc
	statUserTablesAutoanalyzeCount *prometheus.Desc
	statUserTablesTotalSize        *prometheus.Desc
}

func NewPGStatUserTablesCollector(config collectorConfig) (Collector, error) {
	return &PGStatUserTablesCollector{
		log: config.logger,
		statUserTablesSeqScan: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTableSubsystem, "seq_scan"),
			"Number of sequential scans initiated on this table",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statUserTablesSeqTupRead: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTableSubsystem, "seq_tup_read"),
			"Number of live rows fetched by sequential scans",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statUserTablesIdxScan: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTableSubsystem, "idx_scan"),
			"Number of index scans initiated on this table",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statUserTablesIdxTupFetch: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTableSubsystem, "idx_tup_fetch"),
			"Number of live rows fetched by index scans",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statUserTablesNTupIns: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTableSubsystem, "n_tup_ins"),
			"Number of rows inserted",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statUserTablesNTupUpd: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTableSubsystem, "n_tup_upd"),
			"Number of rows updated",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statUserTablesNTupDel: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTableSubsystem, "n_tup_del"),
			"Number of rows deleted",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statUserTablesNTupHotUpd: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTableSubsystem, "n_tup_hot_upd"),
			"Number of rows HOT updated (i.e., with no separate index update required)",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statUserTablesNLiveTup: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTableSubsystem, "n_live_tup"),
			"Estimated number of live rows",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statUserTablesNDeadTup: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTableSubsystem, "n_dead_tup"),
			"Estimated number of dead rows",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statUserTablesNModSinceAnalyze: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTableSubsystem, "n_mod_since_analyze"),
			"Estimated number of rows changed since last analyze",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statUserTablesLastVacuum: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTableSubsystem, "last_vacuum"),
			"Last time at which this table was manually vacuumed (not counting VACUUM FULL)",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statUserTablesLastAutovacuum: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTableSubsystem, "last_autovacuum"),
			"Last time at which this table was vacuumed by the autovacuum daemon",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statUserTablesLastAnalyze: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTableSubsystem, "last_analyze"),
			"Last time at which this table was manually analyzed",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statUserTablesLastAutoanalyze: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTableSubsystem, "last_autoanalyze"),
			"Last time at which this table was analyzed by the autovacuum daemon",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statUserTablesVacuumCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTableSubsystem, "vacuum_count"),
			"Number of times this table has been manually vacuumed (not counting VACUUM FULL)",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statUserTablesAutovacuumCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTableSubsystem, "autovacuum_count"),
			"Number of times this table has been vacuumed by the autovacuum daemon",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statUserTablesAnalyzeCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTableSubsystem, "analyze_count"),
			"Number of times this table has been manually analyzed",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statUserTablesAutoanalyzeCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTableSubsystem, "autoanalyze_count"),
			"Number of times this table has been analyzed by the autovacuum daemon",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
		statUserTablesTotalSize: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, userTableSubsystem, "size_bytes"),
			"Total disk space used by this table, in bytes, including all indexes and TOAST data",
			[]string{"datname", "schemaname", "relname"},
			config.constantLabels,
		),
	}, nil
}

var statUserTablesQuery = `SELECT
		current_database() datname,
		schemaname,
		relname,
		seq_scan,
		seq_tup_read,
		idx_scan,
		idx_tup_fetch,
		n_tup_ins,
		n_tup_upd,
		n_tup_del,
		n_tup_hot_upd,
		n_live_tup,
		n_dead_tup,
		n_mod_since_analyze,
		COALESCE(last_vacuum, '1970-01-01Z') as last_vacuum,
		COALESCE(last_autovacuum, '1970-01-01Z') as last_autovacuum,
		COALESCE(last_analyze, '1970-01-01Z') as last_analyze,
		COALESCE(last_autoanalyze, '1970-01-01Z') as last_autoanalyze,
		vacuum_count,
		autovacuum_count,
		analyze_count,
		autoanalyze_count,
		pg_total_relation_size(relid) as total_size
	FROM
		pg_stat_user_tables`

func (c *PGStatUserTablesCollector) Update(ctx context.Context, instance *instance, ch chan<- prometheus.Metric) error {
	db := instance.getDB()
	rows, err := db.QueryContext(ctx,
		statUserTablesQuery)

	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var datname, schemaname, relname sql.NullString
		var seqScan, seqTupRead, idxScan, idxTupFetch, nTupIns, nTupUpd, nTupDel, nTupHotUpd, nLiveTup, nDeadTup,
			nModSinceAnalyze, vacuumCount, autovacuumCount, analyzeCount, autoanalyzeCount, totalSize sql.NullInt64
		var lastVacuum, lastAutovacuum, lastAnalyze, lastAutoanalyze sql.NullTime

		if err := rows.Scan(&datname, &schemaname, &relname, &seqScan, &seqTupRead, &idxScan, &idxTupFetch, &nTupIns, &nTupUpd, &nTupDel, &nTupHotUpd, &nLiveTup, &nDeadTup, &nModSinceAnalyze, &lastVacuum, &lastAutovacuum, &lastAnalyze, &lastAutoanalyze, &vacuumCount, &autovacuumCount, &analyzeCount, &autoanalyzeCount, &totalSize); err != nil {
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

		seqScanMetric := 0.0
		if seqScan.Valid {
			seqScanMetric = float64(seqScan.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statUserTablesSeqScan,
			prometheus.CounterValue,
			seqScanMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		seqTupReadMetric := 0.0
		if seqTupRead.Valid {
			seqTupReadMetric = float64(seqTupRead.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statUserTablesSeqTupRead,
			prometheus.CounterValue,
			seqTupReadMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		idxScanMetric := 0.0
		if idxScan.Valid {
			idxScanMetric = float64(idxScan.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statUserTablesIdxScan,
			prometheus.CounterValue,
			idxScanMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		idxTupFetchMetric := 0.0
		if idxTupFetch.Valid {
			idxTupFetchMetric = float64(idxTupFetch.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statUserTablesIdxTupFetch,
			prometheus.CounterValue,
			idxTupFetchMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		nTupInsMetric := 0.0
		if nTupIns.Valid {
			nTupInsMetric = float64(nTupIns.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statUserTablesNTupIns,
			prometheus.CounterValue,
			nTupInsMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		nTupUpdMetric := 0.0
		if nTupUpd.Valid {
			nTupUpdMetric = float64(nTupUpd.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statUserTablesNTupUpd,
			prometheus.CounterValue,
			nTupUpdMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		nTupDelMetric := 0.0
		if nTupDel.Valid {
			nTupDelMetric = float64(nTupDel.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statUserTablesNTupDel,
			prometheus.CounterValue,
			nTupDelMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		nTupHotUpdMetric := 0.0
		if nTupHotUpd.Valid {
			nTupHotUpdMetric = float64(nTupHotUpd.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statUserTablesNTupHotUpd,
			prometheus.CounterValue,
			nTupHotUpdMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		nLiveTupMetric := 0.0
		if nLiveTup.Valid {
			nLiveTupMetric = float64(nLiveTup.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statUserTablesNLiveTup,
			prometheus.GaugeValue,
			nLiveTupMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		nDeadTupMetric := 0.0
		if nDeadTup.Valid {
			nDeadTupMetric = float64(nDeadTup.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statUserTablesNDeadTup,
			prometheus.GaugeValue,
			nDeadTupMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		nModSinceAnalyzeMetric := 0.0
		if nModSinceAnalyze.Valid {
			nModSinceAnalyzeMetric = float64(nModSinceAnalyze.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statUserTablesNModSinceAnalyze,
			prometheus.GaugeValue,
			nModSinceAnalyzeMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		lastVacuumMetric := 0.0
		if lastVacuum.Valid {
			lastVacuumMetric = float64(lastVacuum.Time.Unix())
		}
		ch <- prometheus.MustNewConstMetric(
			c.statUserTablesLastVacuum,
			prometheus.GaugeValue,
			lastVacuumMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		lastAutovacuumMetric := 0.0
		if lastAutovacuum.Valid {
			lastAutovacuumMetric = float64(lastAutovacuum.Time.Unix())
		}
		ch <- prometheus.MustNewConstMetric(
			c.statUserTablesLastAutovacuum,
			prometheus.GaugeValue,
			lastAutovacuumMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		lastAnalyzeMetric := 0.0
		if lastAnalyze.Valid {
			lastAnalyzeMetric = float64(lastAnalyze.Time.Unix())
		}
		ch <- prometheus.MustNewConstMetric(
			c.statUserTablesLastAnalyze,
			prometheus.GaugeValue,
			lastAnalyzeMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		lastAutoanalyzeMetric := 0.0
		if lastAutoanalyze.Valid {
			lastAutoanalyzeMetric = float64(lastAutoanalyze.Time.Unix())
		}
		ch <- prometheus.MustNewConstMetric(
			c.statUserTablesLastAutoanalyze,
			prometheus.GaugeValue,
			lastAutoanalyzeMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		vacuumCountMetric := 0.0
		if vacuumCount.Valid {
			vacuumCountMetric = float64(vacuumCount.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statUserTablesVacuumCount,
			prometheus.CounterValue,
			vacuumCountMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		autovacuumCountMetric := 0.0
		if autovacuumCount.Valid {
			autovacuumCountMetric = float64(autovacuumCount.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statUserTablesAutovacuumCount,
			prometheus.CounterValue,
			autovacuumCountMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		analyzeCountMetric := 0.0
		if analyzeCount.Valid {
			analyzeCountMetric = float64(analyzeCount.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statUserTablesAnalyzeCount,
			prometheus.CounterValue,
			analyzeCountMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		autoanalyzeCountMetric := 0.0
		if autoanalyzeCount.Valid {
			autoanalyzeCountMetric = float64(autoanalyzeCount.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statUserTablesAutoanalyzeCount,
			prometheus.CounterValue,
			autoanalyzeCountMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)

		totalSizeMetric := 0.0
		if totalSize.Valid {
			totalSizeMetric = float64(totalSize.Int64)
		}
		ch <- prometheus.MustNewConstMetric(
			c.statUserTablesTotalSize,
			prometheus.GaugeValue,
			totalSizeMetric,
			datnameLabel, schemanameLabel, relnameLabel,
		)
	}

	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}
