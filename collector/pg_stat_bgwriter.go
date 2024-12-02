// Copyright 2021 The Prometheus Authors
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

	"github.com/prometheus/client_golang/prometheus"
)

const bgWriterSubsystem = "stat_bgwriter"

func init() {
	registerCollector(bgWriterSubsystem, defaultEnabled, NewPGStatBGWriterCollector)
}

type PGStatBGWriterCollector struct {
	statBGWriterCheckpointsTimedDesc    *prometheus.Desc
	statBGWriterCheckpointsReqDesc      *prometheus.Desc
	statBGWriterCheckpointsReqTimeDesc  *prometheus.Desc
	statBGWriterCheckpointsSyncTimeDesc *prometheus.Desc
	statBGWriterBuffersCheckpointDesc   *prometheus.Desc
	statBGWriterBuffersCleanDesc        *prometheus.Desc
	statBGWriterMaxwrittenCleanDesc     *prometheus.Desc
	statBGWriterBuffersBackendDesc      *prometheus.Desc
	statBGWriterBuffersBackendFsyncDesc *prometheus.Desc
	statBGWriterBuffersAllocDesc        *prometheus.Desc
	statBGWriterStatsResetDesc          *prometheus.Desc
}

func NewPGStatBGWriterCollector(config collectorConfig) (Collector, error) {
	return &PGStatBGWriterCollector{
		statBGWriterCheckpointsTimedDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, bgWriterSubsystem, "checkpoints_timed_total"),
			"Number of scheduled checkpoints that have been performed",
			[]string{},
			config.constantLabels,
		),
		statBGWriterCheckpointsReqDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, bgWriterSubsystem, "checkpoints_req_total"),
			"Number of requested checkpoints that have been performed",
			[]string{},
			config.constantLabels,
		),
		statBGWriterCheckpointsReqTimeDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, bgWriterSubsystem, "checkpoint_write_time_total"),
			"Total amount of time that has been spent in the portion of checkpoint processing where files are written to disk, in milliseconds",
			[]string{},
			config.constantLabels,
		),
		statBGWriterCheckpointsSyncTimeDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, bgWriterSubsystem, "checkpoint_sync_time_total"),
			"Total amount of time that has been spent in the portion of checkpoint processing where files are synchronized to disk, in milliseconds",
			[]string{},
			config.constantLabels,
		),
		statBGWriterBuffersCheckpointDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, bgWriterSubsystem, "buffers_checkpoint_total"),
			"Number of buffers written during checkpoints",
			[]string{},
			config.constantLabels,
		),
		statBGWriterBuffersCleanDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, bgWriterSubsystem, "buffers_clean_total"),
			"Number of buffers written by the background writer",
			[]string{},
			config.constantLabels,
		),
		statBGWriterMaxwrittenCleanDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, bgWriterSubsystem, "maxwritten_clean_total"),
			"Number of times the background writer stopped a cleaning scan because it had written too many buffers",
			[]string{},
			config.constantLabels,
		),
		statBGWriterBuffersBackendDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, bgWriterSubsystem, "buffers_backend_total"),
			"Number of buffers written directly by a backend",
			[]string{},
			config.constantLabels,
		),
		statBGWriterBuffersBackendFsyncDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, bgWriterSubsystem, "buffers_backend_fsync_total"),
			"Number of times a backend had to execute its own fsync call (normally the background writer handles those even when the backend does its own write)",
			[]string{},
			config.constantLabels,
		),
		statBGWriterBuffersAllocDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, bgWriterSubsystem, "buffers_alloc_total"),
			"Number of buffers allocated",
			[]string{},
			config.constantLabels,
		),
		statBGWriterStatsResetDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, bgWriterSubsystem, "stats_reset_total"),
			"Time at which these statistics were last reset",
			[]string{},
			config.constantLabels,
		),
	}, nil
}

var (
	statBGWriterQuery = `SELECT
		checkpoints_timed
		,checkpoints_req
		,checkpoint_write_time
		,checkpoint_sync_time
		,buffers_checkpoint
		,buffers_clean
		,maxwritten_clean
		,buffers_backend
		,buffers_backend_fsync
		,buffers_alloc
		,stats_reset
	FROM pg_stat_bgwriter;`
)

func (c *PGStatBGWriterCollector) Update(ctx context.Context, instance *instance, ch chan<- prometheus.Metric) error {
	db := instance.getDB()
	row := db.QueryRowContext(ctx,
		statBGWriterQuery)

	var cpt, cpr, bcp, bc, mwc, bb, bbf, ba sql.NullInt64
	var cpwt, cpst sql.NullFloat64
	var sr sql.NullTime

	err := row.Scan(&cpt, &cpr, &cpwt, &cpst, &bcp, &bc, &mwc, &bb, &bbf, &ba, &sr)
	if err != nil {
		return err
	}

	cptMetric := 0.0
	if cpt.Valid {
		cptMetric = float64(cpt.Int64)
	}
	ch <- prometheus.MustNewConstMetric(
		c.statBGWriterCheckpointsTimedDesc,
		prometheus.CounterValue,
		cptMetric,
	)
	cprMetric := 0.0
	if cpr.Valid {
		cprMetric = float64(cpr.Int64)
	}
	ch <- prometheus.MustNewConstMetric(
		c.statBGWriterCheckpointsReqDesc,
		prometheus.CounterValue,
		cprMetric,
	)
	cpwtMetric := 0.0
	if cpwt.Valid {
		cpwtMetric = float64(cpwt.Float64)
	}
	ch <- prometheus.MustNewConstMetric(
		c.statBGWriterCheckpointsReqTimeDesc,
		prometheus.CounterValue,
		cpwtMetric,
	)
	cpstMetric := 0.0
	if cpst.Valid {
		cpstMetric = float64(cpst.Float64)
	}
	ch <- prometheus.MustNewConstMetric(
		c.statBGWriterCheckpointsSyncTimeDesc,
		prometheus.CounterValue,
		cpstMetric,
	)
	bcpMetric := 0.0
	if bcp.Valid {
		bcpMetric = float64(bcp.Int64)
	}
	ch <- prometheus.MustNewConstMetric(
		c.statBGWriterBuffersCheckpointDesc,
		prometheus.CounterValue,
		bcpMetric,
	)
	bcMetric := 0.0
	if bc.Valid {
		bcMetric = float64(bc.Int64)
	}
	ch <- prometheus.MustNewConstMetric(
		c.statBGWriterBuffersCleanDesc,
		prometheus.CounterValue,
		bcMetric,
	)
	mwcMetric := 0.0
	if mwc.Valid {
		mwcMetric = float64(mwc.Int64)
	}
	ch <- prometheus.MustNewConstMetric(
		c.statBGWriterMaxwrittenCleanDesc,
		prometheus.CounterValue,
		mwcMetric,
	)
	bbMetric := 0.0
	if bb.Valid {
		bbMetric = float64(bb.Int64)
	}
	ch <- prometheus.MustNewConstMetric(
		c.statBGWriterBuffersBackendDesc,
		prometheus.CounterValue,
		bbMetric,
	)
	bbfMetric := 0.0
	if bbf.Valid {
		bbfMetric = float64(bbf.Int64)
	}
	ch <- prometheus.MustNewConstMetric(
		c.statBGWriterBuffersBackendFsyncDesc,
		prometheus.CounterValue,
		bbfMetric,
	)
	baMetric := 0.0
	if ba.Valid {
		baMetric = float64(ba.Int64)
	}
	ch <- prometheus.MustNewConstMetric(
		c.statBGWriterBuffersAllocDesc,
		prometheus.CounterValue,
		baMetric,
	)
	srMetric := 0.0
	if sr.Valid {
		srMetric = float64(sr.Time.Unix())
	}
	ch <- prometheus.MustNewConstMetric(
		c.statBGWriterStatsResetDesc,
		prometheus.CounterValue,
		srMetric,
	)

	return nil
}
