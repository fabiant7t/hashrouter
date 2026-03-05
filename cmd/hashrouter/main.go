package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fabiant7t/hashrouter/internal/config"
	"github.com/fabiant7t/hashrouter/internal/server"
	"github.com/fabiant7t/hashrouter/internal/serviceregistry"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	version     = "dev"
	buildDate   = ""
	gitRevision = ""
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.NewFromEnv()

	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		slog.Error("failed to initialize in-cluster config", "error", err)
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		slog.Error("failed to initialize kubernetes clientset", "error", err)
		os.Exit(1)
	}

	registry, err := serviceregistry.New(ctx, clientset, 0)
	if err != nil {
		slog.Error("failed to initialize service registry", "error", err)
		os.Exit(1)
	}

	addr := ":" + getPort()
	appServer := server.New(registry, version)
	srv := &http.Server{
		Addr:              addr,
		Handler:           appServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("hashrouter listening",
		"addr", addr,
		"version", version,
		"buildDate", buildDate,
		"gitRevision", gitRevision,
		"debug", cfg.Debug,
	)

	serverErrCh := make(chan error, 1)
	go func() {
		err := srv.ListenAndServe()
		if err == http.ErrServerClosed {
			err = nil
		}
		serverErrCh <- err
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutdown requested", "reason", ctx.Err())

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}

		if err := <-serverErrCh; err != nil {
			slog.Error("server stopped with error", "error", err)
			os.Exit(1)
		}

		slog.Info("shutdown complete")
	case err := <-serverErrCh:
		if err != nil {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}
}

func getPort() string {
	if port := os.Getenv("PORT"); port != "" {
		return port
	}
	return "8080"
}
