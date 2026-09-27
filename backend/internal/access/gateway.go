// Package access implements the independently deployed authentication and access proxy.
// Package access реализует отдельно развёртываемый прокси аутентификации и доступа.
package access

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lobov/familyquest/backend/internal/domain"
	"github.com/lobov/familyquest/backend/internal/httpapi"
)

type Catalog interface {
	Account(context.Context, string, string) (int64, int64, error)
	Paid(context.Context, int64) (bool, error)
}
type Gateway struct {
	SSO          *SSO
	Version      *httpapi.VersionInfo
	catalog      Catalog
	origin       *url.URL
	secret, cors string
	client       *http.Client
	proxy        *httputil.ReverseProxy
	mu           sync.Mutex
	attempts     map[string]attempt
}
type attempt struct {
	count int
	since time.Time
}

func New(catalog Catalog, origin, secret, cors string) (*Gateway, error) {
	u, e := url.Parse(origin)
	if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || len(secret) < 32 {
		return nil, errors.New("valid private origin and access secret required")
	}
	u.Path = ""
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.ResponseHeaderTimeout = 15 * time.Second
	g := &Gateway{catalog: catalog, origin: u, secret: secret, cors: cors, attempts: map[string]attempt{}, client: &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	g.proxy = &httputil.ReverseProxy{Transport: transport, Rewrite: func(p *httputil.ProxyRequest) {
		p.SetURL(u)
		p.Out.Host = p.In.Host
		cleanHeaders(p.Out.Header)
		p.Out.Header.Set(httpapi.AccessKeyHeader, secret)
	}, ErrorHandler: func(w http.ResponseWriter, r *http.Request, e error) {
		fail(w, 503, "Приложение временно недоступно")
	}}
	return g, nil
}
func cleanHeaders(h http.Header) {
	for k := range h {
		l := strings.ToLower(k)
		if strings.HasPrefix(l, "x-familyquest-access-") || strings.HasPrefix(l, "x-familyquest-internal-") || l == "x-family-id" || l == "x-participant-id" {
			h.Del(k)
		}
	}
}
func (g *Gateway) limit(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	if len(g.attempts) >= 10000 {
		for k, v := range g.attempts {
			if now.Sub(v.since) >= time.Minute {
				delete(g.attempts, k)
			}
		}
		if len(g.attempts) >= 10000 {
			return false
		}
	}
	v := g.attempts[key]
	if now.Sub(v.since) >= time.Minute {
		v = attempt{since: now}
	}
	v.count++
	g.attempts[key] = v
	return v.count <= 5
}
func jsonResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Access-Control-Allow-Origin", g.cors)
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-FamilyQuest, X-FamilyQuest-Confirmation")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	// Reject ambiguous paths before deciding which policy applies.
	// Неоднозначные пути отклоняются до выбора политики доступа.
	if r.URL.RawPath != "" || path.Clean(r.URL.Path) != r.URL.Path || !strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/api/__access") {
		fail(w, 404, "not found")
		return
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if g.SSO != nil && r.URL.Path == "/api/account/backchannel-logout" {
		g.backchannel(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead && !httpapi.SameSiteRequest(r) {
		fail(w, 403, "Недопустимый источник запроса")
		return
	}
	if g.SSO != nil && (r.URL.Path == "/api/account/authorize" || r.URL.Path == "/api/account/callback" || r.URL.Path == "/api/account/exchange" || r.URL.Path == "/api/account/restore" || r.URL.Path == "/api/account/profile") {
		g.sso(w, r)
		return
	}
	if r.URL.Path == "/api/account/login" {
		if g.SSO != nil {
			fail(w, 404, "Используйте единый вход")
			return
		}
		g.login(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/api/config" {
		jsonResponse(w, 200, map[string]any{"saas": true, "sso": g.SSO != nil})
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/api/version" && g.Version != nil {
		response, status, err := g.rpc(r, http.MethodGet, "/api/version", nil)
		var info struct {
			httpapi.VersionInfo
			Access *httpapi.VersionInfo `json:"access"`
		}
		if err != nil || status != 200 || json.Unmarshal(response, &info) != nil {
			fail(w, 503, "Версия приложения временно недоступна")
			return
		}
		info.Access = g.Version
		jsonResponse(w, 200, info)
		return
	}
	if r.Method == http.MethodGet && (r.URL.Path == "/api/version" || r.URL.Path == "/api/health") {
		g.proxy.ServeHTTP(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/api/session/logout" {
		if g.SSO != nil {
			g.logoutSSO(w, r)
			return
		}
		g.proxy.ServeHTTP(w, r)
		return
	}
	response, status, e := g.rpc(r, http.MethodGet, httpapi.IntrospectionPath, nil)
	if e != nil || status >= 500 {
		fail(w, 503, "Проверка доступа временно недоступна")
		return
	}
	if status != 200 {
		fail(w, 401, "Требуется вход в семейный аккаунт")
		return
	}
	var state httpapi.AccessState
	if json.Unmarshal(response, &state) != nil || state.Principal.FamilyID <= 0 || state.Subscription.FamilyID != state.Principal.FamilyID {
		fail(w, 503, "Некорректный ответ проверки доступа")
		return
	}
	if g.SSO != nil {
		c, err := r.Cookie(g.SSO.sessionName())
		if err != nil {
			fail(w, 401, "Повторите единый вход")
			return
		}
		session, ok := g.SSO.Sessions.Get(c.Value)
		if !ok || session.Scope != strconv.FormatInt(state.Principal.FamilyID, 10) {
			fail(w, 401, "Сессия единого входа завершена")
			return
		}
	}
	if state.Subscription.Status != "active" {
		fail(w, 401, "Доступ к семье приостановлен")
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/api/subscription" {
		if !state.Principal.IsParent() {
			fail(w, 403, "Недостаточно прав")
			return
		}
		state.Subscription.Paid, e = g.catalog.Paid(r.Context(), state.Principal.FamilyID)
		if e != nil {
			fail(w, 503, "Информация об оплате временно недоступна")
			return
		}
		state.Subscription.OwnerEmail = ""
		jsonResponse(w, 200, state.Subscription)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/api/session" && !g.limit("profile:"+strconv.FormatInt(state.Principal.FamilyID, 10)) {
		fail(w, 429, "Слишком много попыток. Повторите через минуту.")
		return
	}
	if !state.Subscription.Access && !httpapi.AllowsExpiredRequest(r.Method, r.URL.Path) {
		fail(w, 403, "Доступ к новым действиям приостановлен. Обратитесь к родителю.")
		return
	}
	g.proxy.ServeHTTP(w, r)
}

func (g *Gateway) rpc(r *http.Request, method, route string, body []byte) ([]byte, int, error) {
	u := *g.origin
	u.Path = route
	u.RawPath = ""
	u.RawQuery = ""
	req, e := http.NewRequestWithContext(r.Context(), method, u.String(), bytes.NewReader(body))
	if e != nil {
		return nil, 0, e
	}
	req.Host = r.Host
	for _, k := range []string{"Authorization", "Cookie", "Origin", "Sec-Fetch-Site", "X-FamilyQuest"} {
		if v := r.Header.Get(k); v != "" {
			req.Header.Set(k, v)
		}
	}
	req.Header.Set(httpapi.AccessKeyHeader, g.secret)
	req.Header.Set("Content-Type", "application/json")
	res, e := g.client.Do(req)
	if e != nil {
		return nil, 0, e
	}
	defer res.Body.Close()
	b, e := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	return b, res.StatusCode, e
}
func (g *Gateway) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(405)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil || d.Decode(new(any)) != io.EOF || len(in.Email) > 254 || len(in.Password) > 72 {
		fail(w, 400, "Некорректные данные входа")
		return
	}
	if !g.limit("account:" + strings.ToLower(strings.TrimSpace(in.Email))) {
		fail(w, 429, "Слишком много попыток. Повторите через минуту.")
		return
	}
	family, owner, e := g.catalog.Account(r.Context(), in.Email, in.Password)
	if e != nil {
		if errors.Is(e, domain.ErrUnauthorized) {
			fail(w, 401, "Неверная почта или пароль")
		} else {
			fail(w, 503, "Вход временно недоступен")
		}
		return
	}
	b, _ := json.Marshal(httpapi.AccountSessionInput{FamilyID: family, ParticipantID: owner})
	response, status, e := g.rpc(r, http.MethodPost, httpapi.AccountSessionPath, b)
	if e != nil || status >= 500 {
		fail(w, 503, "Вход временно недоступен")
		return
	}
	if status != 200 {
		fail(w, 401, "Вход в семейный аккаунт недоступен")
		return
	}
	// The origin revokes the previous device; replace the browser cookie too.
	// Приложение отзывает прежнее устройство; удаляем и cookie браузера.
	http.SetCookie(w, &http.Cookie{Name: "familyquest_device", Value: "", Path: "/api", HttpOnly: true, Secure: r.Host != "localhost" && !strings.HasPrefix(r.Host, "127.0.0.1"), SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0)})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, _ = w.Write(response)
}
