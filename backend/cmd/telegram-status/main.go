// telegram-status is a private read-only transport diagnostic. Exit 2 means not ready.
package main

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/lobov/familyquest/backend/internal/access"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	now := time.Now().UTC()
	enabled, worker := os.Getenv("TELEGRAM_ENABLED") == "true", os.Getenv("TELEGRAM_OUTBOX_ENABLED") == "true"
	result := struct {
		Scope          string                          `json:"scope"`
		CheckedAt      time.Time                       `json:"checkedAt"`
		Telegram       bool                            `json:"telegramEnabled"`
		Outbox         bool                            `json:"outboxEnabled"`
		TransportReady bool                            `json:"transportReady"`
		Reasons        []string                        `json:"reasons"`
		Portal         *access.TelegramReadiness       `json:"portal,omitempty"`
		Worker         *access.TelegramOutboxHeartbeat `json:"worker,omitempty"`
	}{Scope: "familyquest_telegram_transport", CheckedAt: now, Telegram: enabled, Outbox: worker, Reasons: []string{}}
	if !enabled {
		result.Reasons = append(result.Reasons, "telegram_disabled")
	}
	if !worker {
		result.Reasons = append(result.Reasons, "outbox_disabled")
	} else {
		heartbeat, err := access.ReadTelegramOutboxHeartbeat(access.TelegramOutboxHeartbeatPath, now)
		result.Worker = &heartbeat
		if err != nil {
			result.Reasons = append(result.Reasons, err.Error())
		}
	}
	portal, err := access.ReadTelegramReadiness(ctx, os.Getenv("TELEGRAM_PORTAL_ORIGIN"), os.Getenv("TELEGRAM_SERVICE_KEY"))
	if err != nil {
		result.Reasons = append(result.Reasons, err.Error())
	} else {
		result.Portal = &portal
		if !portal.Ready {
			result.Reasons = append(result.Reasons, "portal_not_ready")
		}
	}
	result.TransportReady = len(result.Reasons) == 0
	if json.NewEncoder(os.Stdout).Encode(result) != nil {
		os.Exit(1)
	}
	if !result.TransportReady {
		os.Exit(2)
	}
}
