// A Linux oneshot publisher, invoked by the supplied systemd timer.
package main

import (
	"blog-website/backend/internal/seo"
	"blog-website/backend/internal/storage"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func main() {
	force := flag.Bool("force", false, "build even if the current revision is published (frontend release)")
	status := flag.Bool("status", false, "print durable publication state without building")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, logger, *force, *status); err != nil {
		if errors.Is(err, seo.ErrBusy) {
			logger.Info("SEO publisher already running; next timer run will recheck")
			return
		}
		logger.Error("SEO refresh failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger, force, status bool) error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	db, err := storage.Open(dsn)
	if err != nil {
		return err
	}
	pool, err := db.DB()
	if err != nil {
		return err
	}
	defer pool.Close()
	if status {
		readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		p, err := seo.Read(readCtx, db)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(p)
	}
	if runtime.GOOS != "linux" {
		return errors.New("live SEO publication requires Linux; --status is portable")
	}
	frontend := os.Getenv("SEO_FRONTEND_DIR")
	origin, apiOrigin := os.Getenv("VITE_SITE_URL"), os.Getenv("SEO_API_ORIGIN")
	if frontend == "" || origin == "" || apiOrigin == "" {
		return errors.New("SEO_FRONTEND_DIR, VITE_SITE_URL and SEO_API_ORIGIN are required")
	}
	frontend, err = filepath.Abs(frontend)
	if err != nil {
		return err
	}
	timeout := 5 * time.Minute
	if value := os.Getenv("SEO_BUILD_TIMEOUT"); value != "" {
		timeout, err = time.ParseDuration(value)
		if err != nil || timeout < time.Second || timeout > 30*time.Minute {
			return errors.New("SEO_BUILD_TIMEOUT must be between 1s and 30m")
		}
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	worker := seo.Worker{DB: db, Origin: strings.TrimSuffix(origin, "/"), Force: force, Logger: logger,
		Layout: seo.Layout{Releases: os.Getenv("SEO_RELEASES_DIR"), Current: os.Getenv("SEO_CURRENT_LINK"), Assets: os.Getenv("SEO_ASSETS_DIR")},
		Build: func(ctx context.Context, release string) error {
			command := exec.CommandContext(ctx, "npm", "run", "build-only", "--", "--mode", "production", "--outDir", release, "--configLoader", "runner")
			command.Dir = frontend
			// The frontend build does not need database or account credentials.
			for _, entry := range os.Environ() {
				key := strings.SplitN(entry, "=", 2)[0]
				if key != "DATABASE_URL" && key != "ADMIN_PASSWORD" && key != "USER_PASSWORD" {
					command.Env = append(command.Env, entry)
				}
			}
			command.Stdout, command.Stderr = os.Stdout, os.Stderr
			// Kill the npm process group on timeout/termination, including Vite.
			configureCommand(command)
			if err := command.Run(); err != nil {
				return fmt.Errorf("npm production build: %w", err)
			}
			return nil
		},
	}
	return worker.Run(runCtx)
}
