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
			job := domain.NotificationDelivery{ID: "00000000-0000-4000-8000-000000000001", Lease: "00000000-0000-4000-8000-000000000002", Attempts: 4, Recipient: domain.NotificationRecipient{Reference: "tgr_test", BindingID: "tgb_test", SubscriptionID: "00000000-0000-4000-8000-000000000003", Issuer: "https://identity.example", Subject: "parent", ParentID: 2}}
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
				if in["bindingId"] != "tgb_test" || in["issuer"] != job.Recipient.Issuer || in["subject"] != job.Recipient.Subject || in["recipientRef"] != job.Recipient.Reference {
					t.Error("binding identity lost", in)
				}
				if r.URL.Path == "/v1/telegram/notifications/eligibility" {
					if len(in) != 4 {
						t.Error("unsafe eligibility payload", in)
					}
					fmt.Fprint(w, `{"eligible":true}`)
					return
				}
				if r.URL.Path != "/v1/telegram/notifications" {
					t.Error("unexpected Portal route", r.URL.Path)
				}
				if len(in) != 6 || in["eventKey"] != "familyquest-"+job.ID || in["template"] != "activity" || in["recipientRef"] != "tgr_test" {
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
			g.Telegram.Notifications = NewTelegramNotifications()
			if n, e := g.DispatchTelegramNotifications(context.Background(), 1, 1); e != nil || n != 1 {
				t.Fatal(n, e)
			}
			if settlement.Outcome != tc.outcome || settlement.ID != job.ID || settlement.Lease != job.Lease {
				t.Fatal("incorrect settlement", settlement)
			}
		})
	}
}

// UI sessions and files are deliberately absent. Durable Core state and Portal
// binding eligibility are the only sources of delivery authority after restart.
func TestTelegramOutboxChecksDurableAuthorityAfterRestart(t *testing.T) {
	for _, tc := range []struct {
		name              string
		allowed, eligible bool
		outcome, code     string
	}{
		{"active without UI or registry", true, true, "delivered", ""},
		{"role or subscription revoked", false, true, "cancelled", "authorization_revoked"},
		{"stop or old binding", true, false, "cancelled", "portal_denied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			job := domain.NotificationDelivery{ID: "00000000-0000-4000-8000-000000000001", Lease: "00000000-0000-4000-8000-000000000002", Recipient: domain.NotificationRecipient{Reference: "tgr_test", BindingID: "tgb_test", Issuer: "https://id.example", Subject: "parent", ParentID: 2}}
			var settlement domain.NotificationSettlement
			core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case httpapi.NotificationPath + "claim":
					json.NewEncoder(w).Encode(job)
				case httpapi.NotificationPath + "check":
					json.NewEncoder(w).Encode(map[string]bool{"allowed": tc.allowed})
				case httpapi.NotificationPath + "settle":
					var in httpapi.NotificationRequest
					json.NewDecoder(r.Body).Decode(&in)
					settlement = in.Settlement
					fmt.Fprint(w, `{"ok":true}`)
				default:
					t.Error("unexpected Core route", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer core.Close()
			eligibilityCalls, sends := 0, 0
			portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var in map[string]string
				json.NewDecoder(r.Body).Decode(&in)
				if in["sessionToken"] != "" {
					t.Error("UI session dependency")
				}
				switch r.URL.Path {
				case "/v1/telegram/notifications/eligibility":
					eligibilityCalls++
					json.NewEncoder(w).Encode(map[string]bool{"eligible": tc.eligible})
				case "/v1/telegram/notifications":
					sends++
					fmt.Fprint(w, `{"notificationId":"tgn_test"}`)
				default:
					t.Error("unexpected Portal route", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer portal.Close()
			transport, e := NewTelegramPortal(portal.URL, strings.Repeat("p", 40))
			if e != nil {
				t.Fatal(e)
			}
			g, e := New(linkedCatalog{}, core.URL, strings.Repeat("k", 40), "")
			if e != nil {
				t.Fatal(e)
			}
			g.Telegram = NewTelegram(transport, linkedCatalog{}, "", "", true)
			g.Telegram.Notifications = NewTelegramNotifications()
			if n, e := g.DispatchTelegramNotifications(context.Background(), 1, 1); e != nil || n != 1 {
				t.Fatal(n, e)
			}
			if settlement.Outcome != tc.outcome || settlement.ErrorCode != tc.code {
				t.Fatal(settlement)
			}
			if !tc.allowed && eligibilityCalls != 0 {
				t.Fatal("Core denial reached Portal")
			}
			if (!tc.allowed || !tc.eligible) && sends != 0 {
				t.Fatal("revoked recipient sent")
			}
			if tc.allowed && tc.eligible && sends != 1 {
				t.Fatal("restart lost authorized work")
			}
		})
	}
}
