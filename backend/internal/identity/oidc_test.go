package identity

import (
	"context"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/lobov/familyquest/backend/internal/testoidc"
)

func TestSignedOIDCAndClientIsolation(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		claims                  map[string]any
		badState, missingCookie bool
		valid                   bool
	}{
		{name: "valid", valid: true},
		{name: "other audience", claims: map[string]any{"aud": "other-product"}},
		{name: "other issuer", claims: map[string]any{"iss": "https://attacker.example"}},
		{name: "wrong nonce", claims: map[string]any{"nonce": "replayed"}},
		{name: "expired", claims: map[string]any{"exp": time.Now().Add(-time.Hour).Unix()}},
		{name: "wrong authorized party", claims: map[string]any{"azp": "other-product"}},
		{name: "unbound multiple audiences", claims: map[string]any{"aud": []string{"product-a", "product-b"}}},
		{name: "wrong state", badState: true},
		{name: "missing browser binding", missingCookie: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testoidc.New(t, tc.claims)
			l, err := New(context.Background(), Config{Issuer: p.Server.URL, ClientID: "product-a", ClientSecret: "test-secret", RedirectURL: "http://localhost/api/account/callback"})
			if err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			l.Start(w, httptest.NewRequest("GET", "http://localhost/api/account/authorize", nil))
			code, state := p.Code(t, w.Header().Get("Location"))
			if tc.badState {
				state = "forged"
			}
			r := httptest.NewRequest("GET", "http://localhost/api/account/callback?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(state), nil)
			if !tc.missingCookie {
				r.AddCookie(w.Result().Cookies()[0])
			}
			subject, err := l.Complete(httptest.NewRecorder(), r)
			if tc.valid {
				if err != nil || subject.ID != "shared-person" || subject.Issuer != p.Server.URL {
					t.Fatal(subject, err)
				}
			} else if err == nil {
				t.Fatal("accepted invalid identity")
			}
			if _, err = l.Complete(httptest.NewRecorder(), r); err == nil {
				t.Fatal("callback replay accepted")
			}
		})
	}
}
func TestTwoProductsShareIdentityWithoutSharingAudience(t *testing.T) {
	p := testoidc.New(t, nil)
	for _, client := range []string{"product-a", "product-b"} {
		l, e := New(context.Background(), Config{Issuer: p.Server.URL, ClientID: client, ClientSecret: "test-secret", RedirectURL: "http://localhost/api/account/callback"})
		if e != nil {
			t.Fatal(e)
		}
		w := httptest.NewRecorder()
		l.Start(w, httptest.NewRequest("GET", "http://localhost/api/account/authorize", nil))
		code, state := p.Code(t, w.Header().Get("Location"))
		r := httptest.NewRequest("GET", "http://localhost/api/account/callback?code="+code+"&state="+state, nil)
		r.AddCookie(w.Result().Cookies()[0])
		subject, e := l.Complete(httptest.NewRecorder(), r)
		if e != nil || subject.ID != "shared-person" {
			t.Fatal(subject, e)
		}
	}
}
