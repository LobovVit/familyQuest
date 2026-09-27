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

	if cfg.Migrate {
		if err := db.Migrate(ctx); err != nil {
			log.Fatalf("migrate database: %v", err)
		}

	}
	if !cfg.SaaS {
		hasData, err := db.HasAnyData(ctx)
		if err != nil {
			log.Fatalf("check seed state: %v", err)
		}
		seedPath := store.ResolveSeedPath(cfg.SeedFile)
		if !hasData && !cfg.SaaS {
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

	}

	tokens, err := auth.New(cfg.SessionSecret, cfg.SessionTTL)
	if err != nil {
		log.Fatalf("configure sessions: %v", err)
	}
	app := application.New(db, tokens)
	var handler http.Handler = httpapi.NewServer(app, cfg.CORSOrigin)
	if cfg.SaaS {
		platform := store.NewPlatform(db)
		if !cfg.Migrate {
			if err := platform.CheckRuntime(ctx); err != nil {
				log.Fatal(err)
			}
		}
		if cfg.Migrate {
			if err := platform.Migrate(ctx); err != nil {
				log.Fatal(err)
			}
			if err := platform.MigrateFamilies(ctx); err != nil {
				log.Fatal(err)
			}
		}
		tenantHandler := httpapi.NewSaaS(platform, func(ctx context.Context, id int64) (*application.Service, error) {
			repo, err := platform.Family(ctx, id)
			if err != nil {
				return nil, err
			}
			return application.NewForFamily(repo, tokens.ForFamily(id), id), nil
		}, tokens, cfg.CORSOrigin)
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && r.URL.Path == "/api/health" {
				if db.Ping(r.Context()) != nil {
					http.Error(w, "unavailable", 503)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"status":"ok"}`))
				return
			}
			tenantHandler.ServeHTTP(w, r)
		})
	}
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
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
