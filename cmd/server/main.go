package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/diamondsacademy/diamonds/internal/auth"
	"github.com/diamondsacademy/diamonds/internal/config"
	"github.com/diamondsacademy/diamonds/internal/db"
	"github.com/diamondsacademy/diamonds/internal/i18n"
	"github.com/diamondsacademy/diamonds/internal/logger"
	"github.com/diamondsacademy/diamonds/internal/oauth"
	"github.com/diamondsacademy/diamonds/internal/server"
	"github.com/diamondsacademy/diamonds/internal/session"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logger.New(cfg.LogLevel)
	log.Info("boot", slog.String("env", cfg.Env), slog.String("addr", cfg.Addr()))

	conn, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Error("db open failed", slog.String("err", err.Error()))
		os.Exit(1)
	}
	defer conn.Close()

	if err := db.Migrate(conn); err != nil {
		log.Error("migrate failed", slog.String("err", err.Error()))
		os.Exit(1)
	}

	authSvc := auth.NewService(conn)
	if err := authSvc.EnsureAdmin(context.Background(), cfg.AdminEmail, cfg.AdminPassword); err != nil {
		log.Warn("ensure admin failed", slog.String("err", err.Error()))
	}

	if err := i18n.Load("web/static/translations"); err != nil {
		log.Warn("i18n load", slog.String("err", err.Error()))
	}

	sessionDBPath := os.Getenv("SESSION_DB_PATH")
	if sessionDBPath == "" {
		sessionDBPath = cfg.DBPath
	}
	log.Info("session db path", slog.String("path", sessionDBPath))
	sessionConn, err := db.Open(sessionDBPath)
	if err != nil {
		log.Error("session db open failed", slog.String("err", err.Error()))
		os.Exit(1)
	}
	defer sessionConn.Close()
	if err := db.Migrate(sessionConn); err != nil {
		log.Warn("session db migrate", slog.String("err", err.Error()))
	}

	sm := session.New(sessionConn, cfg.SessionLifetime, !cfg.IsDev())

	googleOAuthCfg := oauth.NewGoogleProvider(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL)
	appleProvider := oauth.NewAppleProvider(cfg.AppleTeamID, cfg.AppleServiceID, cfg.AppleKeyID, cfg.ApplePrivateKey, cfg.AppleRedirectURL)
	instagramProvider := oauth.NewInstagramProvider(cfg.InstagramClientID, cfg.InstagramClientSecret, cfg.InstagramRedirectURL)

	r := server.NewRouter(server.Deps{
		Logger:            log,
		DB:                conn,
		SM:                sm,
		AuthSvc:           authSvc,
		GoogleOAuth:       googleOAuthCfg,
		AppleProvider:     appleProvider,
		InstagramProvider: instagramProvider,
	})

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// graceful shutdown
	go func() {
		log.Info("listening", slog.String("addr", cfg.Addr()))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", slog.String("err", err.Error()))
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
