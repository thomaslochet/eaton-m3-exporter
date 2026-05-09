package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"

	"gitlab.synost.net/synost/eaton-m3-exporter/internal/config"
	"gitlab.synost.net/synost/eaton-m3-exporter/internal/exporter"
	"gitlab.synost.net/synost/eaton-m3-exporter/internal/m3"
	"gitlab.synost.net/synost/eaton-m3-exporter/internal/server"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	os.Exit(run())
}

func run() int {
	configPath := flag.String("config", "config.yaml", "path to YAML configuration file")
	showVersion := flag.Bool("version", false, "print version and exit")
	logLevel := flag.String("log-level", "info", "log level: debug, info, warn, error")
	flag.Parse()

	if *showVersion {
		fmt.Printf("eaton-m3-exporter version=%s commit=%s date=%s\n", version, commit, date)
		return 0
	}

	logger, err := newLogger(*logLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid log level: %v\n", err)
		return 2
	}
	slog.SetDefault(logger)

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("load config failed", "error", err)
		return 1
	}

	clients, err := newClients(cfg)
	if err != nil {
		logger.Error("create clients failed", "error", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	for _, client := range clients {
		if err := client.Authenticate(ctx); err != nil {
			logger.Error("initial authentication failed", "target", client.Target().Name, "error", err)
			return 1
		}
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(exporter.NewCollector(clientInterfaces(clients), cfg.ScrapeTimeout.Duration, logger))

	srv := server.New(cfg.ListenAddr, registry, logger)
	if err := srv.Run(ctx); err != nil {
		logger.Error("server failed", "error", err)
		return 1
	}
	return 0
}

func newClients(cfg config.Config) ([]*m3.Client, error) {
	clients := make([]*m3.Client, 0, len(cfg.Targets))
	for _, target := range cfg.Targets {
		client, err := m3.NewClient(m3.Target{
			Name:               target.Name,
			BaseURL:            target.BaseURL,
			Username:           target.Username,
			Password:           target.Password,
			APIVersion:         cfg.APIVersion,
			InsecureSkipVerify: target.InsecureSkipVerify,
		})
		if err != nil {
			return nil, fmt.Errorf("target %q: %w", target.Name, err)
		}
		clients = append(clients, client)
	}
	return clients, nil
}

func clientInterfaces(clients []*m3.Client) []exporter.Client {
	out := make([]exporter.Client, 0, len(clients))
	for _, client := range clients {
		out = append(out, client)
	}
	return out
}

func newLogger(level string) (*slog.Logger, error) {
	var parsed slog.Level
	switch strings.ToLower(level) {
	case "debug":
		parsed = slog.LevelDebug
	case "info":
		parsed = slog.LevelInfo
	case "warn", "warning":
		parsed = slog.LevelWarn
	case "error":
		parsed = slog.LevelError
	default:
		return nil, fmt.Errorf("unsupported level %q", level)
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: parsed})), nil
}
