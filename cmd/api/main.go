package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ashanniwantha/amptudix-health/internal/config"
	"github.com/ashanniwantha/amptudix-health/internal/handler"
	"github.com/ashanniwantha/amptudix-health/internal/logger"
)

func main() {
	config, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
	}

	logger := logger.NewLogger(config.AppEnv)
	logger.Info("starting application", "env", config.AppEnv, "port", config.AppPort)

	// Wire up the router config
	router := handler.RegisterRoutes(handler.Config{
		Logger: logger,
	})

	// HTTP server with timeouts
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", config.AppPort),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start HTTP listener asynchronously
	go func() {
		logger.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("serve shutting down...", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("forced shutdown", "err", err)
		os.Exit(1)
	}

	logger.Info("server stopped safely")
}
