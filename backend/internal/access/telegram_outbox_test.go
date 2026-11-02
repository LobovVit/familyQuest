package access

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lobov/familyquest/backend/internal/domain"
	"github.com/lobov/familyquest/backend/internal/httpapi"
)

func TestTelegramOutboxClassifiesFailuresWithoutDroppingTransientWork(t *testing.T) {
	for _, tc := range []struct {
		name          string
		status        int
		code, outcome string
	}{
		{"outage", 503, "unavailable", "retry"}, {"throttle", 429, "", "retry"},
		{"credential configuration", 401, "unauthorized", "retry"}, {"wrong product", 403, "product_mismatch", "retry"},
		{"revoked consent", 403, "denied", "cancelled"}, {"dedup conflict", 409, "conflict", "failed"},
		{"invalid payload", 400, "invalid_request", "failed"}, {"accepted", 202, "", "delivered"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			job := domain.NotificationDelivery{ID: "00000000-0000-4000-8000-000000000001", Lease: "00000000-0000-4000-8000-000000000002", Attempts: 4, Recipient: domain.NotificationRecipient{Reference: "tgr_test", SubscriptionID: "00000000-0000-4000-8000-000000000003", Issuer: "https://identity.example", Subject: "parent", ParentID: 2}}
			var settlement domain.NotificationSettlement
			core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Error("browser credentials reached private worker call")
				}
				if r.Header.Get(httpapi.AccessKeyHeader) != strings.Repeat("k", 40) {
					t.Error("worker missing Core key")
				}
				var in httpapi.NotificationRequest
				json.NewDecoder(r.Body).Decode(&in)
				if in.FamilyID != 1 {
					t.Error("family scope lost")
				}
				switch r.URL.Path {
				case httpapi.NotificationPath + "claim":
					json.NewEncoder(w).Encode(job)
				case httpapi.NotificationPath + "check":
					fmt.Fprint(w, `{"allowed":true}`)
				case httpapi.NotificationPath + "settle":
					settlement = in.Settlement
					fmt.Fprint(w, `{"ok":true}`)
				default:
					t.Error("unknown Core route", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer core.Close()
			portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var in map[string]string
				json.NewDecoder(r.Body).Decode(&in)
				if len(in) != 3 || in["eventKey"] != "familyquest-"+job.ID || in["template"] != "activity" || in["recipientRef"] != "tgr_test" {
					t.Error("unsafe or unstable notification payload", in)
				}
				w.WriteHeader(tc.status)
				if tc.status == 202 {
					fmt.Fprint(w, `{"notificationId":"tgn_test"}`)
				} else {
					json.NewEncoder(w).Encode(map[string]string{"error": tc.code})
				}
			}))
			defer portal.Close()
			transport, e := NewTelegramPortal(portal.URL, strings.Repeat("p", 40))
			if e != nil {
				t.Fatal(e)
			}
			g, e := New(linkedCatalog{}, core.URL, strings.Repeat("k", 40), "https://family.example")
			if e != nil {
				t.Fatal(e)
			}
			g.Telegram = NewTelegram(transport, linkedCatalog{}, job.Recipient.Issuer, "", true)
			g.Telegram.Notifications, _ = NewTelegramNotifications("")
			if e = g.Telegram.Notifications.set(telegramRecipient{Ref: job.Recipient.Reference, Issuer: job.Recipient.Issuer, Subject: job.Recipient.Subject, SubscriptionID: job.Recipient.SubscriptionID, Family: 1, Parent: 2}, true); e != nil {
				t.Fatal(e)
			}
			if n, e := g.DispatchTelegramNotifications(context.Background(), 1, 1); e != nil || n != 1 {
				t.Fatal(n, e)
			}
			if settlement.Outcome != tc.outcome || settlement.ID != job.ID || settlement.Lease != job.Lease {
				t.Fatal("incorrect settlement", settlement)
			}
		})
	}
}

func TestTelegramOutboxDistinguishesMissingRegistryFromRevocation(t *testing.T) {
	for _, mode := range []string{"missing", "disabled", "new generation"} {
		t.Run(mode, func(t *testing.T) {
			job := domain.NotificationDelivery{ID: "00000000-0000-4000-8000-000000000001", Lease: "00000000-0000-4000-8000-000000000002", Attempts: 1, Recipient: domain.NotificationRecipient{Reference: "tgr_test", SubscriptionID: "00000000-0000-4000-8000-000000000003", Issuer: "https://id.example", Subject: "parent", ParentID: 2}}
			var settlement domain.NotificationSettlement
			core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case httpapi.NotificationPath + "claim":
					json.NewEncoder(w).Encode(job)
				case httpapi.NotificationPath + "settle":
					var in httpapi.NotificationRequest
					json.NewDecoder(r.Body).Decode(&in)
					settlement = in.Settlement
					fmt.Fprint(w, `{"ok":true}`)
				default:
					t.Error("delivery admitted without matching consent")
					w.WriteHeader(403)
				}
			}))
			defer core.Close()
			g, e := New(linkedCatalog{}, core.URL, strings.Repeat("k", 40), "")
			if e != nil {
				t.Fatal(e)
			}
			g.Telegram = NewTelegram(telegramPortalStub{}, linkedCatalog{}, "", "", true)
			g.Telegram.Notifications, e = NewTelegramNotifications(t.TempDir() + "/registry.json")
			if e != nil {
				t.Fatal(e)
			}
			v := telegramRecipient{Ref: job.Recipient.Reference, SubscriptionID: job.Recipient.SubscriptionID, Issuer: job.Recipient.Issuer, Subject: job.Recipient.Subject, Parent: 2, Family: 1}
			switch mode {
			case "disabled":
				e = g.Telegram.Notifications.set(v, false)
			case "new generation":
				v.SubscriptionID = "00000000-0000-4000-8000-000000000004"
				e = g.Telegram.Notifications.set(v, true)
			}
			if e != nil {
				t.Fatal(e)
			}
			// A restart must preserve the evidence of opt-out; an absent file is not that evidence.
			path := g.Telegram.Notifications.path
			g.Telegram.Notifications, e = NewTelegramNotifications(path)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = g.DispatchTelegramNotifications(context.Background(), 1, 1); e != nil {
				t.Fatal(e)
			}
			if mode == "missing" {
				if settlement.Outcome != "retry" || settlement.ErrorCode != "registry_unavailable" {
					t.Fatal("missing file silently lost work", settlement)
				}
			} else if settlement.Outcome != "cancelled" || settlement.ErrorCode != "recipient_revoked" {
				t.Fatal("revocation was not retained", settlement)
			}
		})
	}
}
