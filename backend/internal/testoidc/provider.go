// Package testoidc provides a signed, isolated OIDC provider for protocol tests.
// Package testoidc предоставляет изолированный OIDC provider с подписью для тестов.
package testoidc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

type grant struct{ Client, Nonce, Challenge string }
type Provider struct {
	Server   *httptest.Server
	mu       sync.Mutex
	codes    map[string]grant
	Override map[string]any
	signer   jose.Signer
}

func New(t *testing.T, override map[string]any) *Provider {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: key}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test-key"))
	if err != nil {
		t.Fatal(err)
	}
	p := &Provider{codes: map[string]grant{}, Override: override, signer: signer}
	p.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{"end_session_endpoint": p.Server.URL + "/logout", "issuer": p.Server.URL, "authorization_endpoint": p.Server.URL + "/authorize", "token_endpoint": p.Server.URL + "/token", "jwks_uri": p.Server.URL + "/keys", "id_token_signing_alg_values_supported": []string{"ES256"}})
		case "/keys":
			_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "test-key", Algorithm: "ES256", Use: "sig"}}})
		case "/token":
			_ = r.ParseForm()
			id, secret, ok := r.BasicAuth()
			p.mu.Lock()
			g, found := p.codes[r.Form.Get("code")]
			delete(p.codes, r.Form.Get("code"))
			p.mu.Unlock()
			sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if !found || !ok || secret != "test-secret" || id != g.Client || base64.RawURLEncoding.EncodeToString(sum[:]) != g.Challenge {
				http.Error(w, "invalid grant", 400)
				return
			}
			claims := map[string]any{"iss": p.Server.URL, "sub": "shared-person", "sid": "shared-session", "aud": g.Client, "exp": time.Now().Add(time.Minute).Unix(), "iat": time.Now().Unix(), "nonce": g.Nonce}
			for k, v := range p.Override {
				claims[k] = v
			}
			body, _ := json.Marshal(claims)
			signed, e := signer.Sign(body)
			if e != nil {
				t.Error(e)
				w.WriteHeader(500)
				return
			}
			jwt, e := signed.CompactSerialize()
			if e != nil {
				t.Error(e)
				w.WriteHeader(500)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "provider-only-access-token", "token_type": "Bearer", "id_token": jwt, "expires_in": 60})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(p.Server.Close)
	return p
}
func (p *Provider) Code(t *testing.T, authorization string) (code, state string) {
	t.Helper()
	u, e := url.Parse(authorization)
	if e != nil {
		t.Fatal(e)
	}
	q := u.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("nonce") == "" || q.Get("state") == "" || q.Get("response_type") != "code" {
		t.Fatal("unsafe authorization parameters", q)
	}
	var raw [24]byte
	_, _ = rand.Read(raw[:])
	code = base64.RawURLEncoding.EncodeToString(raw[:])
	p.mu.Lock()
	p.codes[code] = grant{Client: q.Get("client_id"), Nonce: q.Get("nonce"), Challenge: q.Get("code_challenge")}
	p.mu.Unlock()
	return code, q.Get("state")
}

func (p *Provider) Sign(t *testing.T, claims map[string]any) string {
	t.Helper()
	b, e := json.Marshal(claims)
	if e != nil {
		t.Fatal(e)
	}
	v, e := p.signer.Sign(b)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := v.CompactSerialize()
	if e != nil {
		t.Fatal(e)
	}
	return raw
}
