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

	"blog-website/backend/internal/blog"
	"blog-website/backend/internal/music"
	"blog-website/backend/internal/music/provider/netease"
	"blog-website/backend/internal/platform"
	"blog-website/backend/internal/recommendation"
	"blog-website/backend/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := platform.LoadConfig()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var handler http.Handler
	storageName := "postgresql"
	if cfg.Demo {
		handler = platform.NewHandler(blog.NewDemoService(), logger)
		storageName = "in-memory demo (explicit opt-in)"
	} else {
		db, err := storage.Open(cfg.DatabaseURL)
		if err != nil {
			return err
		}
		pool, _ := db.DB()
		defer pool.Close()
		musicService := music.New(db, netease.New(), ctx)
		defer musicService.Stop()
		handler = platform.NewDatabaseHandler(db, cfg, logger, musicService)
		jobsDone := make(chan struct{})
		go func() {
			defer close(jobsDone)
			platform.RunJobs(ctx, musicService, recommendation.Service{DB: db}, logger, cfg.MusicAutoSync)
		}()
		defer func() { stop(); <-jobsDone }()
	}
	server := &http.Server{
		Addr: cfg.Addr, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second,
	}
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	logger.Info("starting API", "addr", cfg.Addr, "storage", storageName)
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return err
		}
		err := <-done
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
