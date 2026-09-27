// Package identity integrates standards-based login without application-specific users.
// Package identity подключает стандартный единый вход без прикладных профилей.
package identity

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type Subject struct{ Issuer, ID, SessionID string }
type Config struct{ Issuer, ClientID, ClientSecret, RedirectURL string }
type flow struct {
	state, nonce, verifier string
	expires                time.Time
}
type Login struct {
	endSession string
	config     Config
	oauth      oauth2.Config
	verifier   *oidc.IDTokenVerifier
	client     *http.Client
	mu         sync.Mutex
	flows      map[string]flow
	secure     bool
	now        func() time.Time
}

func localHTTP(u *url.URL) bool {
	return u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")
}
func validURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !localHTTP(u)) {
		return nil, errors.New("OIDC requires HTTPS or loopback HTTP URLs")
	}
	return u, nil
}
func New(ctx context.Context, c Config) (*Login, error) {
	if _, err := validURL(c.Issuer); err != nil {
		return nil, err
	}
	redirect, err := validURL(c.RedirectURL)
	if err != nil {
		return nil, err
	}
	if c.ClientID == "" || c.ClientSecret == "" {
		return nil, errors.New("OIDC client ID and secret required")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	client := &http.Client{Timeout: 10 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	provider, err := oidc.NewProvider(oidc.ClientContext(ctx, client), c.Issuer)
	if err != nil {
		return nil, err
	}
	var metadata struct {
		Keys       string `json:"jwks_uri"`
		EndSession string `json:"end_session_endpoint"`
	}
	if provider.Claims(&metadata) != nil {
		return nil, errors.New("invalid OIDC discovery")
	}
	if _, err = validURL(metadata.Keys); err != nil {
		return nil, err
	}
	if metadata.EndSession != "" {
		if _, err = validURL(metadata.EndSession); err != nil {
			return nil, err
		}
	}
	endpoint := provider.Endpoint()
	for _, raw := range []string{endpoint.AuthURL, endpoint.TokenURL} {
		if _, err = validURL(raw); err != nil {
			return nil, err
		}
	}
	return &Login{endSession: metadata.EndSession, config: c, oauth: oauth2.Config{ClientID: c.ClientID, ClientSecret: c.ClientSecret, RedirectURL: c.RedirectURL, Endpoint: endpoint, Scopes: []string{oidc.ScopeOpenID}}, verifier: provider.Verifier(&oidc.Config{ClientID: c.ClientID, SupportedSigningAlgs: []string{oidc.RS256, oidc.ES256}}), client: client, flows: map[string]flow{}, secure: redirect.Scheme == "https", now: time.Now}, nil
}
func random() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}
func (l *Login) cookieName() string {
	if l.secure {
		return "__Host-product-login"
	}
	return "product-login-local"
}
func (l *Login) cookie(w http.ResponseWriter, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: l.cookieName(), Value: value, Path: "/", HttpOnly: true, Secure: l.secure, SameSite: http.SameSiteLaxMode, MaxAge: age})
}
func headers(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

// Start stores nonce and PKCE server-side; the browser gets only a random binding.
// Start хранит nonce и PKCE на сервере; браузер получает только случайную привязку.
func (l *Login) Start(w http.ResponseWriter, r *http.Request) {
	headers(w)
	if r.Method != "GET" {
		w.WriteHeader(405)
		return
	}
	id := random()
	f := flow{state: random(), nonce: random(), verifier: oauth2.GenerateVerifier(), expires: l.now().Add(10 * time.Minute)}
	l.mu.Lock()
	for k, v := range l.flows {
		if !l.now().Before(v.expires) {
			delete(l.flows, k)
		}
	}
	if len(l.flows) >= 10000 {
		l.mu.Unlock()
		http.Error(w, "Login temporarily unavailable", 503)
		return
	}
	if prior, err := r.Cookie(l.cookieName()); err == nil {
		delete(l.flows, prior.Value)
	}
	l.flows[id] = f
	l.mu.Unlock()
	l.cookie(w, id, 600)
	http.Redirect(w, r, l.oauth.AuthCodeURL(f.state, oidc.Nonce(f.nonce), oauth2.S256ChallengeOption(f.verifier)), http.StatusFound)
}

// Complete consumes a flow once and verifies signature, issuer, audience and nonce.
// Complete однократно использует вход и проверяет подпись, issuer, audience и nonce.
func (l *Login) Complete(w http.ResponseWriter, r *http.Request) (Subject, error) {
	headers(w)
	fail := errors.New("identity login rejected")
	if r.Method != "GET" {
		return Subject{}, fail
	}
	cookie, err := r.Cookie(l.cookieName())
	if err != nil {
		return Subject{}, fail
	}
	l.cookie(w, "", -1)
	l.mu.Lock()
	f, ok := l.flows[cookie.Value]
	delete(l.flows, cookie.Value)
	l.mu.Unlock()
	q := r.URL.Query()
	if !ok || !l.now().Before(f.expires) || len(q["state"]) != 1 || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(f.state)) != 1 || q.Get("error") != "" || len(q["code"]) != 1 || q.Get("code") == "" {
		return Subject{}, fail
	}
	ctx := oidc.ClientContext(r.Context(), l.client)
	token, err := l.oauth.Exchange(ctx, q.Get("code"), oauth2.VerifierOption(f.verifier))
	if err != nil {
		return Subject{}, fail
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok {
		return Subject{}, fail
	}
	id, err := l.verifier.Verify(ctx, raw)
	if err != nil || id.Subject == "" || len(id.Subject) > 512 || subtle.ConstantTimeCompare([]byte(id.Nonce), []byte(f.nonce)) != 1 {
		return Subject{}, fail
	}
	var claims struct {
		AuthorizedParty string `json:"azp"`
		SessionID       string `json:"sid"`
	}
	if id.Claims(&claims) != nil || (claims.AuthorizedParty != "" && claims.AuthorizedParty != l.config.ClientID) || (len(id.Audience) > 1 && claims.AuthorizedParty != l.config.ClientID) {
		return Subject{}, fail
	}
	return Subject{Issuer: l.config.Issuer, ID: id.Subject, SessionID: claims.SessionID}, nil
}

// ReturnURL never accepts a caller-controlled redirect destination.
// ReturnURL никогда не принимает адрес перенаправления из запроса.
func (l *Login) ReturnURL() string {
	u, _ := url.Parse(l.config.RedirectURL)
	return u.Scheme + "://" + u.Host + "/?sso=complete"
}
func (l *Login) Secure() bool { return l.secure }
func (l *Login) ExpectedHost() string {
	u, _ := url.Parse(l.config.RedirectURL)
	return strings.ToLower(u.Host)
}
