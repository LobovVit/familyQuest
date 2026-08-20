package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lobov/familyquest/backend/internal/application"
	"github.com/lobov/familyquest/backend/internal/auth"
	"github.com/lobov/familyquest/backend/internal/config"
	"github.com/lobov/familyquest/backend/internal/httpapi"
	"github.com/lobov/familyquest/backend/internal/store"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		log.Fatalf("migrate database: %v", err)
	}

	hasData, err := db.HasAnyData(ctx)
	if err != nil {
		log.Fatalf("check seed state: %v", err)
	}
	seedPath := store.ResolveSeedPath(cfg.SeedFile)
	if !hasData {
		imported, err := db.SeedFromBackupFile(ctx, seedPath)
		if err != nil {
			log.Fatalf("seed database from %s: %v", seedPath, err)
		}
		if imported {
			log.Printf("seeded database from %s", seedPath)
		} else {
			log.Printf("seed file %s not found; starting with empty database", seedPath)
		}
	} else {
		log.Printf("database already has data; seed import skipped")
	}

	tokens, err := auth.New(cfg.SessionSecret, cfg.SessionTTL)
	if err != nil {
		log.Fatalf("configure sessions: %v", err)
	}
	app := application.New(db, tokens)
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewServer(app, cfg.CORSOrigin),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("familyQuest API listening on %s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen and serve: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown server: %v", err)
	}
}
