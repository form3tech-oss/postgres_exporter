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

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/form3tech-oss/postgres_exporter/collector"
	"github.com/form3tech-oss/postgres_exporter/config"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	"github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

var (
	c = config.Handler{
		Config: &config.Config{},
	}

	configFile             = kingpin.Flag("config.file", "Postgres exporter configuration file.").Default("postgres_exporter.yml").String()
	webConfig              = kingpinflag.AddFlags(kingpin.CommandLine, ":9187")
	metricsPath            = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").Envar("PG_EXPORTER_WEB_TELEMETRY_PATH").String()
	disableDefaultMetrics  = kingpin.Flag("disable-default-metrics", "Do not include default metrics.").Default("false").Envar("PG_EXPORTER_DISABLE_DEFAULT_METRICS").Bool()
	disableSettingsMetrics = kingpin.Flag("disable-settings-metrics", "Do not include pg_settings metrics.").Default("false").Envar("PG_EXPORTER_DISABLE_SETTINGS_METRICS").Bool()
	autoDiscoverDatabases  = kingpin.Flag("auto-discover-databases", "Whether to discover the databases on a server dynamically. (DEPRECATED)").Default("false").Envar("PG_EXPORTER_AUTO_DISCOVER_DATABASES").Bool()
	queriesPath            = kingpin.Flag("extend.query-path", "Path to custom queries to run. (DEPRECATED)").Default("").Envar("PG_EXPORTER_EXTEND_QUERY_PATH").String()
	onlyDumpMaps           = kingpin.Flag("dumpmaps", "Do not run, simply dump the maps.").Bool()
	onlyHealthCheck        = kingpin.Flag("healthcheck", "Do not run, just return if up and running.").Bool()
	constantLabelsList     = kingpin.Flag("constantLabels", "A list of label=value separated by comma(,). (DEPRECATED)").Default("").Envar("PG_EXPORTER_CONSTANT_LABELS").String()
	excludeDatabases       = kingpin.Flag("exclude-databases", "A list of databases to remove when autoDiscoverDatabases is enabled (DEPRECATED)").Default("").Envar("PG_EXPORTER_EXCLUDE_DATABASES").String()
	includeDatabases       = kingpin.Flag("include-databases", "A list of databases to include when autoDiscoverDatabases is enabled (DEPRECATED)").Default("").Envar("PG_EXPORTER_INCLUDE_DATABASES").String()
	metricPrefix           = kingpin.Flag("metric-prefix", "A metric prefix can be used to have non-default (not \"pg\") prefixes for each of the metrics").Default("pg").Envar("PG_EXPORTER_METRIC_PREFIX").String()
	maxOpenConnections     = kingpin.Flag("max-connections", "the maximum number of opened connections").Default("-1").Envar("PG_MAX_CONNECTIONS").Int()
	maxIdleConnections     = kingpin.Flag("max-idle-connections", "the maximum number of idle connections").Default("-1").Envar("PG_MAX_IDLE_CONNECTIONS").Int()
	collectorTimeout       = kingpin.Flag("collector-timeout", "the single collector scrape timeout").Default("10s").Envar("PG_COLLECTOR_TIMEOUT").Duration()
	logger                 = log.NewNopLogger()
)

// Metric name parts.
const (
	// Namespace for all metrics.
	namespace = "pg"
	// Subsystems.
	exporter = "exporter"
	// The name of the exporter.
	exporterName = "postgres_exporter"
	// Metric label used for static string data thats handy to send to Prometheus
	// e.g. version
	staticLabelName = "static"
	// Metric label used for server identification.
	serverLabelName = "server"
)

func main() {
	kingpin.Version(version.Print(exporterName))
	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger = promlog.New(promlogConfig)

	if *onlyDumpMaps {
		dumpMaps()
		return
	}

	if *onlyHealthCheck {
		healthy, err := runHealthCheck(webConfig)
		if err != nil {
			level.Error(logger).Log("msg", "error running health check", "err", err)
		}
		if healthy {
			level.Info(logger).Log("msg", "health ok")
			os.Exit(0)
		}
		os.Exit(1)
	}

	if err := c.ReloadConfig(*configFile, logger); err != nil {
		// This is not fatal, but it means that auth must be provided for every dsn.
		level.Warn(logger).Log("msg", "Error loading config", "err", err)
	}

	dsns, err := getDataSources()
	if err != nil {
		level.Error(logger).Log("msg", "Failed reading data sources", "err", err.Error())
		os.Exit(1)
	}

	excludedDatabases := strings.Split(*excludeDatabases, ",")
	level.Info(logger).Log("msg", "Excluded databases", "databases", fmt.Sprintf("%v", excludedDatabases))

	if *queriesPath != "" {
		level.Warn(logger).Log("msg", "The extended queries.yaml config is DEPRECATED", "file", *queriesPath)
	}

	if *autoDiscoverDatabases || *excludeDatabases != "" || *includeDatabases != "" {
		level.Warn(logger).Log("msg", "Scraping additional databases via auto discovery is DEPRECATED")
	}

	if *constantLabelsList != "" {
		level.Warn(logger).Log("msg", "Constant labels on all metrics is DEPRECATED")
	}

	opts := []ExporterOpt{
		DisableDefaultMetrics(*disableDefaultMetrics),
		DisableSettingsMetrics(*disableSettingsMetrics),
		AutoDiscoverDatabases(*autoDiscoverDatabases),
		WithUserQueriesPath(*queriesPath),
		WithConstantLabels(*constantLabelsList),
		ExcludeDatabases(excludedDatabases),
		IncludeDatabases(*includeDatabases),
		WithMaxOpenConnections(*maxOpenConnections),
		WithMaxIdleConnections(*maxIdleConnections),
	}

	exporter := NewExporter(dsns, opts...)

	reg := prometheus.NewRegistry()

	reg.MustRegister(
		version.NewCollector(exporterName),
		exporter,
	)

	// TODO(@sysadmind): Remove this with multi-target support. We are removing multiple DSN support
	dsn := ""
	if len(dsns) > 0 {
		dsn = dsns[0]
	}

	collOpts := []collector.Option{
		collector.WithConstantLabels(parseConstLabels(*constantLabelsList)),
		collector.WithMaxIdleConnections(*maxIdleConnections),
		collector.WithMaxOpenConnections(*maxOpenConnections),
		collector.WithScrapeTimeout(*collectorTimeout),
	}

	if *maxOpenConnections >= 0 {
		collOpts = append(collOpts, collector.WithMaxOpenConnections(*maxOpenConnections))
	}
	if *maxIdleConnections >= 0 {
		collOpts = append(collOpts, collector.WithMaxIdleConnections(*maxIdleConnections))
	}

	pgColl, err := collector.NewPostgresCollector(
		logger,
		excludedDatabases,
		dsn,
		[]string{},
		collOpts...,
	)
	if err != nil {
		level.Error(logger).Log("msg", "Failed to create PostgresCollector", "err", err.Error())
	} else {
		reg.MustRegister(pgColl)
	}

	http.Handle(*metricsPath, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	if *metricsPath != "/" && *metricsPath != "" {
		landingConfig := web.LandingConfig{
			Name:        "Postgres Exporter",
			Description: "Prometheus PostgreSQL server Exporter",
			Version:     version.Info(),
			Links: []web.LandingLinks{
				{
					Address: *metricsPath,
					Text:    "Metrics",
				},
			},
		}
		landingPage, err := web.NewLandingPage(landingConfig)
		if err != nil {
			level.Error(logger).Log("err", err)
			os.Exit(1)
		}
		http.Handle("/", landingPage)
	}

	srv := &http.Server{}
	srv.RegisterOnShutdown(func() {
		level.Info(logger).Log("msg", "gracefully shutting down HTTP server")
		exporter.servers.Close()
		pgColl.Close()
	})

	go func() {
		if err := web.ListenAndServe(srv, webConfig, logger); !errors.Is(err, http.ErrServerClosed) {
			level.Error(logger).Log("msg", "running HTTP server", "err", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownRelease()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		level.Error(logger).Log("msg", "during HTTP server shut down", "err", err)
		os.Exit(1)
	}
	level.Info(logger).Log("msg", "HTTP server gracefully shut down")
}

func runHealthCheck(webConfig *web.FlagConfig) (bool, error) {
	if len(*webConfig.WebListenAddresses) == 0 {
		return false, errors.New("no listen addresses to run the request to")
	}
	addr := (*webConfig.WebListenAddresses)[0]
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return false, err
	}
	if host == "" {
		host = "localhost"
	}
	url := fmt.Sprintf("http://%s:%s/", host, port)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}

	defer resp.Body.Close()
	return resp.StatusCode == 200, nil
}
