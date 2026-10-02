package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"blog-website/backend/internal/blog"
	"blog-website/backend/internal/music"
	"blog-website/backend/internal/music/playback"
	"blog-website/backend/internal/music/provider/bilibili"
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
	var media *http.Server
	var mediaListener net.Listener
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
		var player *playback.Service
		if cfg.MusicPlayback {
			if _, err := exec.LookPath(cfg.YTDLP); err != nil {
				return errors.New("MUSIC_YTDLP executable unavailable")
			}
			if _, err := exec.LookPath(cfg.FFmpeg); err != nil {
				return errors.New("MUSIC_FFMPEG executable unavailable")
			}
			var neteaseResolver playback.Resolver = netease.NewAudioResolver()
			if cfg.NeteaseCookieFile != "" {
				account, err := netease.NewAccountResolver(cfg.NeteaseCookieFile, musicService, netease.AccountLimits{PerMinute: cfg.AccountPerMinute, RejectThreshold: cfg.AccountRejectThreshold, Cooldown: cfg.AccountCooldown})
				if err != nil {
					return errors.New("NetEase private credential file unavailable or invalid")
				}
				neteaseResolver = playback.FallbackResolver{Public: neteaseResolver, Account: account, Policy: cfg.PlaybackPolicy}
			}
			options := cfg.PlaybackOptions
			options.FFmpeg = cfg.FFmpeg
			options.ForceTranscode = cfg.MusicForceTranscode
			options.Control = musicService
			player = playback.New(ctx, playback.Resolvers{"bilibili": bilibili.Resolver{Binary: cfg.YTDLP}, "netease": neteaseResolver}, musicService, options)
			defer player.Close()
			media = &http.Server{Addr: cfg.MediaAddr, Handler: platform.NewMusicStreamHandler(player, cfg), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 * 1024}
			mediaListener, err = net.Listen("tcp", cfg.MediaAddr)
			if err != nil {
				return err
			}
			defer mediaListener.Close()
			defer media.Close()
		}
		handler = platform.NewDatabaseHandler(db, cfg, logger, musicService, player)
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
	defer server.Close()
	done := make(chan error, 2)
	if media != nil {
		go func() { done <- media.Serve(mediaListener) }()
	}
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
