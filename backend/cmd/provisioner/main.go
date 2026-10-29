// Provisioner runs only on the private provisioning network, with operator DB credentials.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lobov/familyquest/backend/internal/domain"
	"github.com/lobov/familyquest/backend/internal/provisioning"
	"github.com/lobov/familyquest/backend/internal/store"
)

type registrar struct {
	platform *store.Platform
	coreRole string
}

func (r registrar) RegisterFamily(ctx context.Context, issuer, subject string, input domain.FamilyRegistration) (int64, error) {
	return r.platform.RegisterFamily(ctx, issuer, subject, input, r.coreRole)
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := store.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	role := os.Getenv("CORE_DATABASE_ROLE")
	if role == "" {
		log.Fatal("CORE_DATABASE_ROLE is required")
	}
	handler, err := provisioning.Handler(registrar{store.NewPlatform(db), role}, os.Getenv("OIDC_ISSUER"), os.Getenv("PROVISIONER_SHARED_SECRET"))
	if err != nil {
		log.Fatal(err)
	}
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8083"
	}
	server := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 16}
	go func() {
		log.Print("private family provisioner listening")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	end, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = server.Shutdown(end)
}
