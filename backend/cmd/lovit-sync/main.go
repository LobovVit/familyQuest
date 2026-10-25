// lovit-sync is a separately enabled reconciliation worker, never a payment sender.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/lobov/familyquest/backend/internal/lovit"
	"github.com/lobov/familyquest/backend/internal/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	interval := 60
	if raw := os.Getenv("LOVIT_SYNC_SECONDS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 10 || n > 3600 {
			log.Fatal("LOVIT_SYNC_SECONDS must be 10..3600")
		}
		interval = n
	}
	operator := os.Getenv("OPERATOR_ID")
	if strings.TrimSpace(operator) == "" {
		log.Fatal("OPERATOR_ID required")
	}
	client, err := lovit.New(os.Getenv("LOVIT_ORIGIN"), os.Getenv("LOVIT_ENTITLEMENT_TOKEN"))
	if err != nil {
		log.Fatal("LOVIT origin and product credential required")
	}
	db, err := store.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("database unavailable")
	}
	defer db.Close()
	// Migrations/bindings/grants are an explicit deployment operation, not startup work.
	catalog := store.NewPlatform(db)
	for {
		count, err := lovit.SyncPass(ctx, catalog, client, operator)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Print("LOVIT reconciliation incomplete; previous access expiries retained")
		}
		log.Printf("LOVIT bindings synchronized: %d", count)
		timer := time.NewTimer(time.Duration(interval) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
