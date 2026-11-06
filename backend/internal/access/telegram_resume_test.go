package access

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type resumePortal struct {
	out               telegramExchange
	exchanges, checks int
	denied            bool
	allowFresh        bool
	outage            bool
	onCheck           func()
}

func (p *resumePortal) Call(_ context.Context, route string, _ any, output any) error {
	if route == "exchange" {
		p.exchanges++
		if p.exchanges > 1 && !p.allowFresh {
			return telegramPortalError{status: 409}
		}
	} else if route == "session" {
		p.checks++
		if p.onCheck != nil {
			p.onCheck()
		}
		if p.outage {
			return telegramPortalError{status: 503}
		}
		if p.denied {
			return telegramPortalError{status: 401}
		}
	}
	b, _ := json.Marshal(p.out)
	return json.Unmarshal(b, output)
}

func TestTelegramReloadResumeBoundToVerifiedLaunch(t *testing.T) {
	for _, scenario := range []string{"resume", "fresh launch", "Portal outage", "no cookie", "different cookie", "different launch", "revoked", "Portal expired", "expired", "logout during introspection", "unlink selected identity"} {
		t.Run(scenario, func(t *testing.T) {
			portal := &resumePortal{out: telegramExchange{SessionToken: "synthetic-server-token", ExpiresAt: time.Now().Add(time.Hour), Principal: TelegramPrincipal{ProductID: "familyquest", BotID: "familyquest", IdentityState: "unlinked", RecipientRef: "fixture"}}}
			tg := NewTelegram(portal, nil, "https://id.example", "https://portal.example", true)
			g := &Gateway{Telegram: tg}
			call := func(cookie *http.Cookie, launch string) *httptest.ResponseRecorder {
				body, _ := json.Marshal(map[string]string{"initData": launch})
				r := httptest.NewRequest("POST", "https://family.example/api/telegram/exchange", strings.NewReader(string(body)))
				if cookie != nil {
					r.AddCookie(cookie)
				}
				w := httptest.NewRecorder()
				g.telegram(w, r)
				return w
			}
			first := call(nil, "already-verified-launch")
			if first.Code != 200 {
				t.Fatal(first.Code)
			}
			cookie := first.Result().Cookies()[0]
			launch := "already-verified-launch"
			want := 200
			switch scenario {
			case "fresh launch":
				launch = "new-verified-launch"
				portal.allowFresh = true
				portal.out.Principal.RecipientRef = "other-verified-recipient"
				old := tg.sessions[cookie.Value]
				old.Family = 1
				old.Subject = "old-owner"
				tg.sessions[cookie.Value] = old
			case "Portal outage":
				portal.outage = true
				want = 503
			case "no cookie":
				cookie = nil
				want = 409
			case "different cookie":
				cookie = &http.Cookie{Name: telegramCookie, Value: "not-the-verified-cookie"}
				want = 409
			case "different launch":
				launch = "another-account-launch"
				want = 409
			case "revoked":
				portal.denied = true
				want = 401
			case "Portal expired":
				portal.out.ExpiresAt = time.Now().Add(-time.Second)
				want = 401
			case "expired":
				s := tg.sessions[cookie.Value]
				s.Expires = time.Now().Add(-time.Second)
				tg.sessions[cookie.Value] = s
				want = 409
			case "logout during introspection":
				portal.onCheck = func() { tg.remove(cookie.Value) }
				want = 401
			case "unlink selected identity":
				s := tg.sessions[cookie.Value]
				s.Subject = "selected-owner"
				s.Issuer = "https://id.example"
				s.Family = 1
				tg.sessions[cookie.Value] = s
				want = 401
			}
			result := call(cookie, launch)
			if result.Code != want {
				t.Fatalf("got %d want %d", result.Code, want)
			}
			if scenario == "fresh launch" {
				newCookie := result.Result().Cookies()[len(result.Result().Cookies())-1]
				fresh := tg.sessions[newCookie.Value]
				if fresh.Family != 0 || fresh.Subject != "" || newCookie.Value == cookie.Value {
					t.Fatal("new launch retained old selection")
				}
				if _, exists := tg.sessions[cookie.Value]; exists {
					t.Fatal("old launch remains usable")
				}
			}
			if scenario == "resume" {
				if !tg.sessions[cookie.Value].Expires.Equal(portal.out.ExpiresAt) {
					t.Fatal("resume changed absolute expiry")
				}
				if portal.exchanges != 1 || portal.checks != 2 {
					t.Fatal("resume must introspect without another one-use exchange")
				}
				if len(result.Result().Cookies()) != 0 {
					t.Fatal("resume must not extend/rotate cookie expiry")
				}
				// A failed new-account launch must invalidate the old local context.
				if call(cookie, "invalid-new-account").Code != 409 || call(cookie, launch).Code != 409 {
					t.Fatal("account switch retained prior session")
				}
			}
		})
	}
}
