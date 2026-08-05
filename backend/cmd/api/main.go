package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lobov/familyquest/backend/internal/config"
	"github.com/lobov/familyquest/backend/internal/httpapi"
	"github.com/lobov/familyquest/backend/internal/store"
)

func main() {
	cfg := config.Load()

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
	seededFromBackup := false
	if !hasData {
		seedPath := store.ResolveSeedPath(cfg.SeedFile)
		imported, err := db.SeedFromBackupFile(ctx, seedPath)
		if err != nil {
			log.Fatalf("seed database from %s: %v", seedPath, err)
		}
		if imported {
			seededFromBackup = true
			log.Printf("seeded database from %s", seedPath)
		}
	}
	if !seededFromBackup {
		if err := db.Seed(ctx); err != nil {
			log.Fatalf("seed default data: %v", err)
		}
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewServer(db, cfg.CORSOrigin),
		ReadHeaderTimeout: 5 * time.Second,
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
