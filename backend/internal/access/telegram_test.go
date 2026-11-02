package access

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lobov/familyquest/backend/internal/httpapi"
)

type telegramPortalStub struct {
	out telegramExchange
	err error
}

func (p telegramPortalStub) Call(_ context.Context, _ string, _ any, out any) error {
	if p.err != nil {
		return p.err
	}
	if out != nil {
		b, _ := json.Marshal(p.out)
		return json.Unmarshal(b, out)
	}
	return nil
}
func TestTelegramSessionContractClosesOnMismatch(t *testing.T) {
	expiry := time.Now().Add(time.Hour)
	good := telegramExchange{ExpiresAt: expiry, Principal: TelegramPrincipal{ProductID: "familyquest", BotID: "familyquest", IdentityState: "linked", Issuer: "https://id.example", Subject: "parent", RecipientRef: "tgr_test"}}
	session := telegramSession{Token: "server-only", Expires: expiry, Family: 1, Issuer: "https://id.example", Subject: "parent"}
	cases := map[string]func(*telegramExchange){
		"product":           func(v *telegramExchange) { v.Principal.ProductID = "lovit-ritm" },
		"bot":               func(v *telegramExchange) { v.Principal.BotID = "lovit-ritm" },
		"issuer":            func(v *telegramExchange) { v.Principal.Issuer = "https://other.example" },
		"subject":           func(v *telegramExchange) { v.Principal.Subject = "another-person" },
		"unlink":            func(v *telegramExchange) { v.Principal.IdentityState = "unlinked" },
		"expiry":            func(v *telegramExchange) { v.ExpiresAt = time.Now().Add(-time.Minute) },
		"extension":         func(v *telegramExchange) { v.ExpiresAt = expiry.Add(time.Hour) },
		"missing recipient": func(v *telegramExchange) { v.Principal.RecipientRef = "" },
	}
	tg := NewTelegram(telegramPortalStub{out: good}, nil, "https://id.example", "https://portal.example/telegram", true)
	if _, e := tg.validate(context.Background(), session); e != nil {
		t.Fatal(e)
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			bad := good
			change(&bad)
			tg.Portal = telegramPortalStub{out: bad}
			if _, e := tg.validate(context.Background(), session); e == nil {
				t.Fatal("accepted invalid Portal identity")
			}
		})
	}
	tg.Portal = telegramPortalStub{err: telegramPortalError{status: 503}}
	if _, e := tg.validate(context.Background(), session); e == nil {
		t.Fatal("Portal outage allowed access")
	}
}
func TestTelegramFamilyRotationCannotResurrectRevokedSession(t *testing.T) {
	tg := NewTelegram(nil, nil, "", "", true)
	s := telegramSession{Token: "test", Expires: time.Now().Add(time.Hour)}
	r := httptest.NewRequest("POST", "https://family.example/api/telegram/family", nil)
	w := httptest.NewRecorder()
	if e := tg.replace(w, r, s, false); e != nil {
		t.Fatal(e)
	}
	cookie := w.Result().Cookies()[0]
	r.AddCookie(cookie)
	if !cookie.HttpOnly || !cookie.Secure || cookie.Path != "/api" {
		t.Fatal("unsafe cookie")
	}
	tg.remove(cookie.Value)
	if e := tg.replace(httptest.NewRecorder(), r, s, true); e == nil {
		t.Fatal("in-flight family selection restored a logged-out session")
	}
}
func TestTelegramPortalRefusesRedirectAndInvalidConfiguration(t *testing.T) {
	for _, origin := range []string{"http://portal.example", "https://user:secret@portal.example", "https://portal.example/base", "https://portal.example?key=x"} {
		if _, e := NewTelegramPortal(origin, strings.Repeat("p", 40)); e == nil {
			t.Fatal("unsafe origin", origin)
		}
	}
	called := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer redirect.Close()
	p, e := NewTelegramPortal(redirect.URL, strings.Repeat("p", 40))
	if e != nil {
		t.Fatal(e)
	}
	if e = p.Call(context.Background(), "session", map[string]string{"sessionToken": "private"}, nil); e == nil || called {
		t.Fatal("Portal credentials followed redirect")
	}
}
func TestTelegramDoesNotHijackOrdinaryWebSession(t *testing.T) {
	r := httptest.NewRequest("GET", "https://family.example/api/participants", nil)
	r.AddCookie(&http.Cookie{Name: telegramCookie, Value: "old"})
	if isTelegramRequest(r) {
		t.Fatal("Telegram cookie changed normal web authentication")
	}
	r.Header.Set("X-FamilyQuest-Client", "telegram")
	if !isTelegramRequest(r) {
		t.Fatal("missing explicit Telegram request mode")
	}
}

func TestTelegramOptOutAttemptsBothAuthorities(t *testing.T) {
	for _, mode := range []string{"Portal unavailable", "Core unavailable"} {
		t.Run(mode, func(t *testing.T) {
			coreCalls, portalCalls := 0, 0
			core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				coreCalls++
				var in httpapi.NotificationRequest
				json.NewDecoder(r.Body).Decode(&in)
				if r.URL.Path != httpapi.NotificationPath+"revoke" || in.FamilyID != 1 || in.Recipient.Reference != "tgr_test" {
					t.Error("incorrect durable revoke", in)
				}
				if mode == "Core unavailable" {
					w.WriteHeader(503)
					return
				}
				w.Write([]byte(`{"ok":true}`))
			}))
			defer core.Close()
			portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				portalCalls++
				var in map[string]any
				json.NewDecoder(r.Body).Decode(&in)
				if r.URL.Path != "/v1/telegram/notifications/consent" || in["enabled"] != false {
					t.Error("incorrect remote revoke", in)
				}
				if mode == "Portal unavailable" {
					w.WriteHeader(503)
					return
				}
				w.Write([]byte(`{"ok":true}`))
			}))
			defer portal.Close()
			transport, e := NewTelegramPortal(portal.URL, strings.Repeat("p", 40))
			if e != nil {
				t.Fatal(e)
			}
			tg := NewTelegram(transport, nil, "https://id.example", "https://portal.example/telegram", true)
			tg.Notifications = NewTelegramNotifications()
			r := httptest.NewRequest("POST", "https://family.example/api/telegram/consent", strings.NewReader(`{"enabled":false}`))
			r.Header.Set("X-FamilyQuest", "1")
			r.Header.Set("Origin", "https://family.example")
			entry := httptest.NewRecorder()
			if e = tg.replace(entry, r, telegramSession{Token: "server-only", RecipientRef: "tgr_test", Family: 1, Expires: time.Now().Add(time.Hour)}, false); e != nil {
				t.Fatal(e)
			}
			r.AddCookie(entry.Result().Cookies()[0])
			g, e := New(fakeCatalog{}, core.URL, strings.Repeat("k", 40), "https://family.example")
			if e != nil {
				t.Fatal(e)
			}
			g.Telegram = tg
			w := httptest.NewRecorder()
			g.ServeHTTP(w, r)
			if w.Code != 503 {
				t.Fatal("partial opt-out failure hidden", w.Code)
			}
			if coreCalls != 1 || portalCalls != 1 {
				t.Fatal("opt-out skipped authority", coreCalls, portalCalls)
			}
		})
	}
}
