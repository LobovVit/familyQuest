package identity

import (
	"context"
	"github.com/lobov/familyquest/backend/internal/testoidc"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSignedBackchannel(t *testing.T) {
	p := testoidc.New(t, nil)
	l, e := New(context.Background(), Config{Issuer: p.Server.URL, ClientID: "product-a", ClientSecret: "test-secret", RedirectURL: "http://localhost/callback"})
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"valid", "wrong audience", "nonce", "missing event", "expired", "future issued", "no subject"} {
		t.Run(name, func(t *testing.T) {
			c := map[string]any{"iss": p.Server.URL, "sub": "person", "sid": "session", "aud": "product-a", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix(), "jti": "event-id", "events": map[string]any{"http://schemas.openid.net/event/backchannel-logout": map[string]any{}}}
			switch name {
			case "wrong audience":
				c["aud"] = "product-b"
			case "nonce":
				c["nonce"] = ""
			case "missing event":
				delete(c, "events")
			case "expired":
				c["exp"] = time.Now().Add(-time.Hour).Unix()
			case "future issued":
				c["iat"] = time.Now().Add(time.Hour).Unix()
			case "no subject":
				delete(c, "sub")
				delete(c, "sid")
			}
			r := httptest.NewRequest("POST", "http://localhost/backchannel", strings.NewReader(url.Values{"logout_token": {p.Sign(t, c)}}.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			_, err := l.VerifyLogout(r)
			if (err == nil) != (name == "valid") {
				t.Fatal("logout validation", err)
			}
		})
	}
	u, e := l.LogoutURL()
	if e != nil {
		t.Fatal(e)
	}
	parsed, _ := url.Parse(u)
	if parsed.Query().Get("post_logout_redirect_uri") != "http://localhost/" || parsed.Query().Get("client_id") != "product-a" {
		t.Fatal(u)
	}
}
func TestSessionRevocationAndCallbackRace(t *testing.T) {
	s := NewSessions()
	subject := Subject{Issuer: "issuer", ID: "person", SessionID: "sid"}
	id, e := s.Add(subject, "family-1")
	if e != nil {
		t.Fatal(e)
	}
	other, e := s.Add(Subject{Issuer: "issuer", ID: "person", SessionID: "other-device"}, "family-1")
	if e != nil {
		t.Fatal(e)
	}
	s.Revoke(subject)
	if _, ok := s.Get(id); ok {
		t.Fatal("revoked session accepted")
	}
	if _, ok := s.Get(other); !ok {
		t.Fatal("unrelated device revoked")
	}
	if _, e = s.Add(subject, "family-1"); e == nil {
		t.Fatal("callback resurrected revoked session")
	}
	s.mu.Lock()
	v := s.active[other]
	v.Expires = time.Now().Add(-time.Second)
	s.active[other] = v
	s.mu.Unlock()
	if _, ok := s.Get(other); ok {
		t.Fatal("expired session accepted")
	}
}

func TestPersistentRememberedSessions(t *testing.T) {
	path := t.TempDir() + "/sessions.json"
	s, err := NewPersistentSessions(path)
	if err != nil {
		t.Fatal(err)
	}
	sub := Subject{Issuer: "issuer", ID: "person", SessionID: "remembered"}
	id, err := s.AddFor(sub, "7", 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewPersistentSessions(path)
	if err != nil {
		t.Fatal(err)
	}
	v, ok := reloaded.Get(id)
	if !ok || time.Until(v.Expires) < 29*24*time.Hour {
		t.Fatal("remembered session lost")
	}
	reloaded.Revoke(sub)
	again, err := NewPersistentSessions(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := again.Get(id); ok {
		t.Fatal("revocation lost on restart")
	}
	if _, err := again.Add(sub, "7"); err == nil {
		t.Fatal("revoked sid resurrected")
	}
}
