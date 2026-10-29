package access

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lobov/familyquest/backend/internal/httpapi"
	"github.com/lobov/familyquest/backend/internal/identity"
	"github.com/lobov/familyquest/backend/internal/testoidc"
)

type linkedCatalog struct{ fakeCatalog }

func (linkedCatalog) PortalFamily(context.Context, string, string) (int64, error) { return 1, nil }

func (linkedCatalog) Identity(context.Context, string, string) (int64, int64, error) {
	return 1, 2, nil
}

func TestSSORememberedEntryAndRevocation(t *testing.T) {
	p := testoidc.New(t, nil)
	key := strings.Repeat("s", 40)
	core := httptest.NewServer(httpapi.RequireAccess(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case httpapi.AccountSessionPath:
			fmt.Fprint(w, `{"token":"internal-owner"}`)
		case "/api/participants":
			fmt.Fprint(w, `[{"id":2,"familyId":1,"role":"parent","active":true,"name":"Test"}]`)
		case "/api/session":
			if r.Header.Get("Authorization") != "Bearer internal-owner" {
				t.Error("missing internal authorization")
			}
			w.WriteHeader(401) // The core still owns PIN verification.
		default:
			http.NotFound(w, r)
		}
	}), key))
	defer core.Close()
	for _, days := range []int{0, 30, 60} {
		t.Run(strconv.Itoa(days), func(t *testing.T) {
			l, err := identity.New(context.Background(), identity.Config{Issuer: p.Server.URL, ClientID: "familyquest", ClientSecret: "test-secret", RedirectURL: "https://family.example/api/account/callback"})
			if err != nil {
				t.Fatal(err)
			}
			g, err := New(linkedCatalog{}, core.URL, key, "https://family.example")
			if err != nil {
				t.Fatal(err)
			}
			g.SSO = NewSSO(l, linkedCatalog{})
			path := t.TempDir() + "/sessions.json"
			g.SSO.Sessions, err = identity.NewPersistentSessions(path)
			if err != nil {
				t.Fatal(err)
			}
			call := func(method, route string, cookie *http.Cookie) *httptest.ResponseRecorder {
				r := httptest.NewRequest(method, "https://family.example"+route, strings.NewReader(`{"participantId":2,"pin":""}`))
				if cookie != nil {
					r.AddCookie(cookie)
				}
				r.Header.Set("Origin", "https://family.example")
				r.Header.Set("X-FamilyQuest", "1")
				w := httptest.NewRecorder()
				g.ServeHTTP(w, r)
				return w
			}
			start := call("GET", "/api/account/authorize?remember="+strconv.Itoa(days), nil)
			code, state := p.Code(t, start.Header().Get("Location"))
			callback := call("GET", "/api/account/callback?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(state)+"&remember=999", start.Result().Cookies()[0])
			if callback.Code != 303 {
				t.Fatal(callback.Code, callback.Body.String())
			}
			var handoffCookie *http.Cookie
			for _, c := range callback.Result().Cookies() {
				if c.Name == g.SSO.name() {
					handoffCookie = c
				}
			}
			exchange := call("POST", "/api/account/exchange", handoffCookie)
			if exchange.Code != 200 || strings.Contains(exchange.Body.String(), "internal-owner") {
				t.Fatal(exchange.Code, "invalid exchange")
			}
			var sessionCookie *http.Cookie
			for _, c := range exchange.Result().Cookies() {
				if c.Name == g.SSO.sessionName() {
					sessionCookie = c
				}
			}
			want := identity.Subject{RememberDays: days}
			if sessionCookie == nil || sessionCookie.MaxAge != want.CookieMaxAge() || !sessionCookie.Secure || !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteLaxMode {
				t.Fatal("incorrect session cookie")
			}
			if replay := call("POST", "/api/account/exchange", handoffCookie); replay.Code != 401 {
				t.Fatal("handoff replay")
			}
			g.SSO.Sessions, err = identity.NewPersistentSessions(path)
			if err != nil {
				t.Fatal(err)
			}
			session, ok := g.SSO.Sessions.Get(sessionCookie.Value)
			if !ok || time.Until(session.Expires) < want.SessionTTL()-time.Minute {
				t.Fatal("duration lost after restart")
			}
			if restore := call("POST", "/api/account/restore", sessionCookie); restore.Code != 200 {
				t.Fatal("restore failed", restore.Code)
			}
			if profile := call("POST", "/api/account/profile", sessionCookie); profile.Code != 401 {
				t.Fatal("PIN bypassed")
			}
			g.SSO.Sessions.Revoke(session.Subject)
			g.SSO.Sessions, err = identity.NewPersistentSessions(path)
			if err != nil {
				t.Fatal(err)
			}
			if restore := call("POST", "/api/account/restore", sessionCookie); restore.Code != 401 {
				t.Fatal("revocation lost")
			}
		})
	}
}
