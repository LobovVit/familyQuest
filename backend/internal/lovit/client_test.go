package lovit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientScopeFreshnessAndNoRedirect(t *testing.T) {
	until := time.Now().Add(time.Hour)
	base := Entitlement{ProductID: "familyquest", SubscriptionID: "sub_test", SubjectRef: "familyquest:7", Access: "active", Paid: true, ValidUntil: &until, Revision: 1, Features: map[string]bool{"children_unlimited": true}, EvaluatedAt: time.Now()}
	for _, tc := range []struct {
		name   string
		modify func(*Entitlement)
		bad    bool
	}{{"valid", func(*Entitlement) {}, false}, {"other product", func(e *Entitlement) { e.ProductID = "other" }, true}, {"other family", func(e *Entitlement) { e.SubjectRef = "familyquest:8" }, true}, {"other subscription", func(e *Entitlement) { e.SubscriptionID = "sub_other" }, true}, {"stale", func(e *Entitlement) { e.EvaluatedAt = time.Now().Add(-time.Hour) }, true}, {"unpaid", func(e *Entitlement) { e.Paid = false }, true}, {"no unlimited right", func(e *Entitlement) { e.Features = map[string]bool{} }, true}, {"permanent", func(e *Entitlement) { e.ValidUntil = nil }, true}} {
		t.Run(tc.name, func(t *testing.T) {
			e := base
			tc.modify(&e)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("t", 40) || r.Header.Get("Cookie") != "" || r.URL.Path != "/v1/entitlements/check" {
					t.Error("request")
				}
				_ = json.NewEncoder(w).Encode(e)
			}))
			defer server.Close()
			client, err := New(server.URL, strings.Repeat("t", 40))
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Check(context.Background(), "familyquest:7", "sub_test")
			if (err != nil) != tc.bad {
				t.Fatal(err)
			}
		})
	}
	var leaked bool
	destination := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { leaked = true }))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 302) }))
	defer redirect.Close()
	c, _ := New(redirect.URL, strings.Repeat("t", 40))
	if _, err := c.Check(context.Background(), "familyquest:7", "sub_test"); err == nil || leaked {
		t.Fatal("redirect leaked credential")
	}
}
