// Explicit local administration. No public account creation or implicit migration.
package main

import (
	"blog-website/backend/internal/auth"
	"blog-website/backend/internal/music"
	"blog-website/backend/internal/music/provider"
	"blog-website/backend/internal/storage"
	"blog-website/backend/migrations"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"io"
	"os"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 {
		return errors.New("usage: go run ./cmd/manage migrate|create-admin|import-music <file.json>")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	switch os.Args[1] {
	case "import-music":
		if len(os.Args) != 3 {
			return errors.New("import-music requires a JSON file")
		}
		file, err := os.Open(os.Args[2])
		if err != nil {
			return errors.New("cannot read import file")
		}
		defer file.Close()
		var payload struct {
			Source struct {
				ID         string  `json:"id"`
				Provider   string  `json:"provider"`
				ExternalID string  `json:"externalId"`
				Title      string  `json:"title"`
				SourceURL  string  `json:"sourceUrl"`
				EmbedURL   *string `json:"embedUrl"`
			} `json:"source"`
			Tracks []provider.Track `json:"tracks"`
		}
		decoder := json.NewDecoder(io.LimitReader(file, 4*1024*1024+1))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&payload) != nil || decoder.Decode(new(any)) != io.EOF || payload.Tracks == nil {
			return errors.New("invalid import JSON (source and explicit tracks array required)")
		}
		db, err := storage.Open(dsn)
		if err != nil {
			return err
		}
		pool, _ := db.DB()
		defer pool.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		v := payload.Source
		if err := music.Import(ctx, db, music.Source{ID: v.ID, Provider: v.Provider, ExternalID: v.ExternalID, Title: v.Title, SourceURL: v.SourceURL, EmbedURL: v.EmbedURL}, payload.Tracks); err != nil {
			return errors.New("import failed; validate source, tracks, migration and active sync status")
		}
		fmt.Println("music snapshot imported")
		return nil
	case "migrate":
		source, err := iofs.New(migrations.Files, ".")
		if err != nil {
			return err
		}
		m, err := migrate.NewWithSourceInstance("iofs", source, dsn)
		if err != nil {
			return errors.New("migration connection failed; verify DATABASE_URL")
		}
		defer m.Close()
		if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return errors.New("migration failed; inspect schema_migrations dirty/version before retrying")
		}
		fmt.Println("migrations applied")
		return nil
	case "create-admin":
		username, password := os.Getenv("ADMIN_USERNAME"), os.Getenv("ADMIN_PASSWORD")
		os.Unsetenv("ADMIN_PASSWORD")
		db, err := storage.Open(dsn)
		if err != nil {
			return err
		}
		pool, _ := db.DB()
		defer pool.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := auth.CreateUser(ctx, db, username, password); err != nil {
			return errors.New("admin creation failed: use a unique username (1–100 characters), a 12–72 byte password, and migrate first")
		}
		fmt.Println("admin created")
		return nil
	default:
		return errors.New("unknown command")
	}
}
