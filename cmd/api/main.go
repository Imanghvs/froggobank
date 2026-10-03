package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Imanghvs/froggobank/internal/account/adapters/httpapi"
	accountpostgres "github.com/Imanghvs/froggobank/internal/account/adapters/postgres"
	"github.com/Imanghvs/froggobank/internal/account/application"
	"github.com/Imanghvs/froggobank/internal/platform/config"
	"github.com/Imanghvs/froggobank/internal/platform/database"
	"github.com/Imanghvs/froggobank/internal/platform/logging"
	"github.com/Imanghvs/froggobank/internal/server"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 10 * time.Second
	databaseTimeout   = 5 * time.Second
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	logger, err := logging.New(cfg.LogLevel, os.Stdout)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}

	logger.Info(
		"application starting",
		"http_port", cfg.HTTPPort,
		"log_level", cfg.LogLevel,
	)

	pool, err := connectDatabase(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	accountRepository := accountpostgres.New(pool)
	accountService := application.New(accountRepository)
	accountHandler := httpapi.New(accountService)

	router := server.NewRouter(logger, pool, accountHandler)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	serverErr := make(chan error, 1)

	go func() {
		logger.Info("HTTP server listening", "address", httpServer.Addr)
		serverErr <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil

	case <-signalCtx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		shutdownTimeout,
	)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	logger.Info("HTTP server stopped!")

	return nil
}

func connectDatabase(databaseURL string) (*pgxpool.Pool, error) {
	startupCtx, startupCancel := context.WithTimeout(
		context.Background(),
		databaseTimeout,
	)
	defer startupCancel()

	pool, err := database.NewPool(startupCtx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	if err := pool.Ping(startupCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}

	return pool, nil
}
