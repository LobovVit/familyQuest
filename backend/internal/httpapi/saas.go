package httpapi

import (
	"context"
	"github.com/lobov/familyquest/backend/internal/application"
	"github.com/lobov/familyquest/backend/internal/domain"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type FamilyServices func(context.Context, int64) (*application.Service, error)
type familyContextKey struct{}
type SaaS struct {
	platform application.PlatformGateway
	resolve  FamilyServices
	tokens   application.Tokens
	cors     string
	mu       sync.Mutex
	attempts map[string]attempt
}
type attempt struct {
	count int
	since time.Time
}

func NewSaaS(platform application.PlatformGateway, resolve FamilyServices, tokens application.Tokens, cors string) http.Handler {
	return &SaaS{platform: platform, resolve: resolve, tokens: tokens, cors: cors, attempts: map[string]attempt{}}
}
func (s *SaaS) limit(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if len(s.attempts) > 10000 {
		for k, v := range s.attempts {
			if now.Sub(v.since) > time.Minute {
				delete(s.attempts, k)
			}
		}
		if len(s.attempts) > 10000 {
			return false
		}
	}
	v := s.attempts[key]
	if now.Sub(v.since) >= time.Minute {
		v = attempt{since: now}
	}
	v.count++
	s.attempts[key] = v
	return v.count <= 5
}
func (s *SaaS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == "OPTIONS" {
		w.Header().Set("Access-Control-Allow-Origin", s.cors)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-FamilyQuest, X-FamilyQuest-Confirmation")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.WriteHeader(204)
		return
	}
	if r.URL.Path == "/api/account/login" {
		s.accountLogin(w, r)
		return
	}
	if r.Method == "GET" && r.URL.Path == "/api/config" {
		writeJSON(w, 200, map[string]bool{"saas": true})
		return
	}
	var family int64
	if r.Header.Get("Authorization") != "" {
		token, e := parseBearer(r.Header.Get("Authorization"))
		if e != nil {
			respond(w, nil, e)
			return
		}
		p, e := s.tokens.Parse(token)
		if e != nil || p.FamilyID <= 0 || p.ConfirmedUntil != 0 {
			respond(w, nil, domain.ErrUnauthorized)
			return
		}
		family = p.FamilyID
	} else if c, e := r.Cookie(deviceCookie); e == nil {
		parts := strings.SplitN(c.Value, ".", 2)
		if len(parts) == 2 {
			family, _ = strconv.ParseInt(parts[0], 10, 64)
		}
	}
	if family <= 0 {
		if r.URL.Path == "/api/session/logout" && r.Method == "POST" && sameSiteRequest(r) {
			setDeviceCookie(w, r, "")
			writeJSON(w, 200, map[string]string{"status": "signed-out"})
			return
		}
		respond(w, nil, domain.ErrUnauthorized)
		return
	}
	service, e := s.resolve(r.Context(), family)
	if e != nil {
		respond(w, nil, e)
		return
	}
	r = r.WithContext(context.WithValue(r.Context(), familyContextKey{}, family))
	server := &Server{store: service, corsOrigin: s.cors, mux: http.NewServeMux()}
	p, e := server.requestPrincipal(w, r)
	if e != nil {
		respond(w, nil, e)
		return
	}
	if p.FamilyID != family {
		respond(w, nil, domain.ErrUnauthorized)
		return
	}
	subscription, e := s.platform.Subscription(r.Context(), family)
	if e != nil {
		respond(w, nil, e)
		return
	}
	if r.URL.Path == "/api/subscription" && r.Method == "GET" {
		if !p.IsParent() {
			respond(w, nil, domain.ErrForbidden)
			return
		}
		subscription.OwnerEmail = ""
		respond(w, subscription, nil)
		return
	}
	if r.Method == "POST" && r.URL.Path == "/api/session" && !s.limit("profile:"+strconv.FormatInt(family, 10)) {
		writeError(w, 429, "Слишком много попыток. Повторите через минуту.")
		return
	}
	if !subscription.Access && r.Method != "GET" && r.Method != "HEAD" && !strings.HasPrefix(r.URL.Path, "/api/session") && !strings.HasSuffix(r.URL.Path, "/answers") && !strings.HasSuffix(r.URL.Path, "/finish") {
		writeError(w, 403, "Доступ к новым действиям приостановлен. Обратитесь к родителю.")
		return
	}
	server.routes()
	server.ServeHTTP(w, r)
}
func (s *SaaS) accountLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	if !sameSiteRequest(r) {
		respond(w, nil, domain.ErrForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if decodeJSON(r, &in) != nil || len(in.Email) > 254 || len(in.Password) > 72 {
		respond(w, nil, domain.ErrInvalidInput)
		return
	}
	if !s.limit("account:" + strings.ToLower(strings.TrimSpace(in.Email))) {
		writeError(w, 429, "Слишком много попыток. Повторите через минуту.")
		return
	}
	family, id, e := s.platform.Account(r.Context(), in.Email, in.Password)
	if e != nil {
		respond(w, nil, e)
		return
	}
	app, e := s.resolve(r.Context(), family)
	if e != nil {
		respond(w, nil, e)
		return
	}
	result, e := app.AccountSession(r.Context(), id)
	if e != nil {
		respond(w, nil, e)
		return
	}
	// Revoke the prior cookie before replacing a session, including another family.
	// Перед заменой сессии отзываем прежнее устройство, в том числе другой семьи.
	if c, err := r.Cookie(deviceCookie); err == nil {
		parts := strings.SplitN(c.Value, ".", 2)
		if len(parts) == 2 {
			old, _ := strconv.ParseInt(parts[0], 10, 64)
			if previous, err := s.resolve(r.Context(), old); err == nil {
				if err = previous.ForgetDevice(r.Context(), parts[1]); err != nil {
					respond(w, nil, err)
					return
				}
			}
		}
	}
	setDeviceCookie(w, r, "")
	respond(w, result, nil)
}
