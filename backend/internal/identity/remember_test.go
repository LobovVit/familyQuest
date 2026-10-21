package identity

import (
	"context"
	"github.com/lobov/familyquest/backend/internal/testoidc"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestRememberChoiceBoundToSignedFlow(t *testing.T) {
	p := testoidc.New(t, nil)
	for _, tc := range []struct {
		query string
		days  int
	}{{"", 0}, {"0", 0}, {"1", 30}, {"30", 30}, {"60", 60}} {
		t.Run(tc.query, func(t *testing.T) {
			l, err := New(context.Background(), Config{Issuer: p.Server.URL, ClientID: "product", ClientSecret: "test-secret", RedirectURL: "http://localhost/callback"})
			if err != nil {
				t.Fatal(err)
			}
			start := httptest.NewRecorder()
			l.Start(start, httptest.NewRequest("GET", "http://localhost/login?remember="+tc.query, nil))
			code, state := p.Code(t, start.Header().Get("Location"))
			r := httptest.NewRequest("GET", "http://localhost/callback?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(state)+"&remember=999", nil)
			r.AddCookie(start.Result().Cookies()[0])
			sub, err := l.Complete(httptest.NewRecorder(), r)
			if err != nil || sub.RememberDays != tc.days {
				t.Fatal(sub, err)
			}
			ttl, age := 12*time.Hour, 0
			if tc.days > 0 {
				ttl = time.Duration(tc.days) * 24 * time.Hour
				age = int(ttl.Seconds())
			}
			if sub.SessionTTL() != ttl || sub.CookieMaxAge() != age {
				t.Fatal("duration/cookie mismatch")
			}
		})
	}
}
func TestRejectInvalidRememberBeforeOIDC(t *testing.T) {
	p := testoidc.New(t, nil)
	l, err := New(context.Background(), Config{Issuer: p.Server.URL, ClientID: "product", ClientSecret: "test-secret", RedirectURL: "http://localhost/callback"})
	if err != nil {
		t.Fatal(err)
	}
	for i, q := range []string{"remember=-1", "remember=31", "remember=365", "remember=30&remember=60", "remember=garbage"} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			w := httptest.NewRecorder()
			l.Start(w, httptest.NewRequest("GET", "http://localhost/login?"+q, nil))
			if w.Code != 400 || w.Header().Get("Location") != "" || len(w.Result().Cookies()) != 0 {
				t.Fatal(w.Code)
			}
		})
	}
}
