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

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ashanniwantha/amptudix-health/internal/auth"
	"github.com/ashanniwantha/amptudix-health/internal/client/openwearables"
	"github.com/ashanniwantha/amptudix-health/internal/config"
	"github.com/ashanniwantha/amptudix-health/internal/database"
	"github.com/ashanniwantha/amptudix-health/internal/health"
	"github.com/ashanniwantha/amptudix-health/internal/logger"
	"github.com/ashanniwantha/amptudix-health/internal/redis"
	"github.com/ashanniwantha/amptudix-health/internal/user"
)

func main() {
	// 1. Config — fail fast if anything required is missing.
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	// 2. Logger.
	log := logger.NewLogger(cfg.App.Env)
	log.Info("starting", "env", cfg.App.Env, "port", cfg.App.Port)

	// 3. Root context cancelled on SIGINT/SIGTERM. Everything downstream
	//    (DB ping, Redis ping) inherits it.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 4. Infrastructure.
	pool, err := database.NewPool(ctx, cfg.DB)
	if err != nil {
		log.Error("database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb, err := redis.NewClient(ctx, cfg.Redis)
	if err != nil {
		log.Error("redis", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := rdb.Close(); err != nil {
			log.Warn("redis close", "error", err)
		}
	}()

	// 5. Auth toolkit — RSA key pair + token generator.
	priv, err := auth.ParseRSAPrivateKey(cfg.Auth.PrivateKeyPath)
	if err != nil {
		log.Error("parse RSA private key", "error", err)
		os.Exit(1)
	}
	pub, err := auth.ParseRSAPublicKey(cfg.Auth.PublicKeyPath)
	if err != nil {
		log.Error("parse RSA public key", "error", err)
		os.Exit(1)
	}
	tokenGen := auth.NewTokenGenerator(priv, pub, cfg.Auth.AccessTTL)

	// 6. Open Wearables client — replaces the old stubOW.
	owClient := openwearables.NewClient(
		cfg.OpenWearables.APIURL,
		cfg.OpenWearables.APIKey,
		cfg.OpenWearables.AppID,
		cfg.OpenWearables.AppSecret,
	)

	// 7. Feature wiring: repository → service → handler.
	txManager := database.NewTxManager(pool)

	userRepo := user.NewRepository()
	userSvc := user.NewService(userRepo, txManager, pool, owClient)
	userHandler := user.NewHandler(userSvc)

	healthSvc := health.NewService(userSvc, owClient)
	healthHandler := health.NewHandler(healthSvc)

	// 8. Router.
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	r.Route("/v1", func(r chi.Router) {
		// Public: registration (and login, when you add it).
		userHandler.RegisterRoutes(r)

		// Protected: everything that needs a JWT.
		r.Group(func(r chi.Router) {
			r.Use(auth.Authenticate(tokenGen, log))

			// Kotlin app calls this to get SDK tokens.
			userHandler.RegisterProtectedRoutes(r)

			// Go backend calls these after the SDK has synced.
			healthHandler.RegisterRoutes(r)
		})
	})

	// 9. HTTP server with timeouts.
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.App.Port),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// 10. Serve in a goroutine.
	go func() {
		log.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server", "error", err)
			stop() // unblock main so deferred closes run
		}
	}()

	// 11. Block until SIGINT/SIGTERM.
	<-ctx.Done()
	log.Info("shutting down")

	// 12. Graceful shutdown with a hard deadline.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("forced shutdown", "error", err)
		os.Exit(1)
	}

	log.Info("stopped cleanly")
}
