// Access is the public API gateway; the application origin stays private.
// Access — публичный шлюз API; основное приложение остаётся закрытым.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lobov/familyquest/backend/internal/access"
	"github.com/lobov/familyquest/backend/internal/buildinfo"
	"github.com/lobov/familyquest/backend/internal/httpapi"
	"github.com/lobov/familyquest/backend/internal/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, e := store.Open(ctx, os.Getenv("DATABASE_URL"))
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()
	catalog := store.NewPlatform(db)
	if e = catalog.CheckAccessRuntime(ctx); e != nil {
		log.Fatal(e)
	}
	gateway, e := access.New(catalog, os.Getenv("CORE_ORIGIN"), os.Getenv("ACCESS_SHARED_SECRET"), os.Getenv("CORS_ORIGIN"))
	if e != nil {
		log.Fatal(e)
	}
	info := httpapi.VersionInfo{Version: buildinfo.Version, Commit: buildinfo.Commit, BuiltAt: buildinfo.BuiltAt, StartedAt: time.Now().UTC().Format(time.RFC3339)}
	gateway.Version = &info
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/access/version" && r.Method == http.MethodGet {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(info)
			return
		}
		if r.URL.Path == "/api/health" && r.Method == http.MethodGet {
			c, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			if db.Ping(c) != nil {
				http.Error(w, "unavailable", 503)
				return
			}
		}
		gateway.ServeHTTP(w, r)
	})
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8082"
	}
	server := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 16}
	go func() {
		log.Printf("access version=%s commit=%s listening on %s", buildinfo.Version, buildinfo.Commit, addr)
		if e := server.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			log.Fatal(e)
		}
	}()
	<-ctx.Done()
	end, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(end)
}
