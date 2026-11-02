package access

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/lobov/familyquest/backend/internal/domain"
	"github.com/lobov/familyquest/backend/internal/httpapi"
)

type NotificationFamilyCatalog interface {
	NotificationFamilies(context.Context, int64) ([]int64, error)
}

func (g *Gateway) notificationRPC(ctx context.Context, route string, in httpapi.NotificationRequest, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.origin.String(), nil)
	if err != nil {
		return err
	}
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	body, status, err := g.rpc(req, http.MethodPost, httpapi.NotificationPath+route, b)
	if err != nil {
		return errors.New("notification Core unavailable")
	}
	if status != 200 {
		return errors.New("notification Core rejected request")
	}
	if out != nil && json.Unmarshal(body, out) != nil {
		return errors.New("invalid notification Core response")
	}
	return nil
}

// DispatchTelegramNotifications runs independently of the request that completed a task.
// A lost response/ack leaves the leased row in PostgreSQL; reclaim retries the same event key.
func (g *Gateway) DispatchTelegramNotifications(ctx context.Context, family int64, limit int) (int, error) {
	if g.Telegram == nil || g.Telegram.Notifications == nil {
		return 0, nil
	}
	if limit < 1 || limit > 20 {
		return 0, errors.New("invalid notification batch size")
	}
	done := 0
	for done < limit && ctx.Err() == nil {
		var job *domain.NotificationDelivery
		if err := g.notificationRPC(ctx, "claim", httpapi.NotificationRequest{FamilyID: family}, &job); err != nil {
			return done, err
		}
		if job == nil {
			return done, nil
		}
		if err := g.dispatchNotification(ctx, family, *job); err != nil {
			return done, err
		}
		done++
	}
	return done, ctx.Err()
}
func (g *Gateway) dispatchNotification(ctx context.Context, family int64, job domain.NotificationDelivery) error {
	n := g.Telegram.Notifications
	n.deliveryMu.Lock()
	defer n.deliveryMu.Unlock()
	settlement := domain.NotificationSettlement{ID: job.ID, Lease: job.Lease, Outcome: "retry", ErrorCode: "transport"}
	deliver := func() {
		actual, _, err := g.Telegram.Catalog.Identity(ctx, job.Recipient.Issuer, job.Recipient.Subject)
		if errors.Is(err, domain.ErrUnauthorized) || err == nil && actual != family {
			settlement.Outcome = "cancelled"
			settlement.ErrorCode = "recipient_revoked"
			return
		}
		if err != nil {
			settlement.ErrorCode = "identity_unavailable"
			return
		}
		var check struct {
			Allowed bool `json:"allowed"`
		}
		if err = g.notificationRPC(ctx, "check", httpapi.NotificationRequest{FamilyID: family, ID: job.ID, Lease: job.Lease}, &check); err != nil {
			return
		}
		if !check.Allowed {
			settlement.Outcome = "cancelled"
			settlement.ErrorCode = "authorization_revoked"
			return
		}
		bound := notificationBinding(job.Recipient)
		var eligibility struct {
			Eligible bool `json:"eligible"`
		}
		err = g.Telegram.Portal.Call(ctx, "notifications/eligibility", bound, &eligibility)
		if err == nil && !eligibility.Eligible {
			err = telegramPortalError{status: 403, code: "denied"}
		}
		var accepted struct {
			ID string `json:"notificationId"`
		}
		if err == nil {
			err = g.Telegram.Portal.Call(ctx, "notifications", map[string]string{"recipientRef": bound.RecipientRef, "bindingId": bound.BindingID, "issuer": bound.Issuer, "subject": bound.Subject, "eventKey": "familyquest-" + job.ID, "template": "activity"}, &accepted)
		}
		if err != nil {
			var rejected telegramPortalError
			if errors.As(err, &rejected) {
				settlement.ErrorCode = "portal_unavailable"
				switch {
				case rejected.status == 403 && rejected.code == "denied":
					settlement.Outcome = "cancelled"
					settlement.ErrorCode = "portal_denied"
				case rejected.status == 409:
					settlement.Outcome = "failed"
					settlement.ErrorCode = "portal_conflict"
				case rejected.status == 400:
					settlement.Outcome = "failed"
					settlement.ErrorCode = "portal_invalid"
				}
			}
			return
		}
		if !strings.HasPrefix(accepted.ID, "tgn_") || len(accepted.ID) > 128 {
			settlement.ErrorCode = "invalid_response"
			return
		}
		settlement.Outcome = "delivered"
		settlement.ErrorCode = ""
		settlement.PortalID = accepted.ID
	}
	deliver()
	// Cancellation/transport failure of settlement intentionally leaves the lease recoverable.
	if err := g.notificationRPC(ctx, "settle", httpapi.NotificationRequest{FamilyID: family, Settlement: settlement}, nil); err != nil {
		return err
	}
	if settlement.Outcome == "retry" || settlement.Outcome == "failed" {
		log.Printf("telegram outbox family=%d event=%s outcome=%s code=%s attempt=%d", family, job.ID, settlement.Outcome, settlement.ErrorCode, job.Attempts)
	}
	return nil
}
func (g *Gateway) RunTelegramOutbox(ctx context.Context, catalog NotificationFamilyCatalog) {
	var after int64
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		ids, err := catalog.NotificationFamilies(ctx, after)
		if err != nil {
			log.Print("telegram outbox catalog unavailable")
		} else {
			for _, id := range ids {
				if ctx.Err() != nil {
					return
				}
				batch, cancel := context.WithTimeout(ctx, 30*time.Second)
				_, err = g.DispatchTelegramNotifications(batch, id, 10)
				cancel()
				if err != nil {
					log.Print("telegram outbox dispatch interrupted; durable work retained")
				}
				after = id
			}
			if len(ids) < 50 {
				after = 0
			}
		}
		timer.Reset(5 * time.Second)
	}
}
