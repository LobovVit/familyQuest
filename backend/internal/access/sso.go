package access

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/lobov/familyquest/backend/internal/domain"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lobov/familyquest/backend/internal/httpapi"
	"github.com/lobov/familyquest/backend/internal/identity"
)

type IdentityCatalog interface {
	Identity(context.Context, string, string) (int64, int64, error)
}
type handoff struct {
	remember bool
	subject  identity.Subject
	expires  time.Time
}
type SSO struct {
	Sessions *identity.Sessions
	Login    *identity.Login
	Catalog  IdentityCatalog
	mu       sync.Mutex
	pending  map[string]handoff
}

func NewSSO(login *identity.Login, catalog IdentityCatalog) *SSO {
	return &SSO{Sessions: identity.NewSessions(), Login: login, Catalog: catalog, pending: map[string]handoff{}}
}
func (s *SSO) name() string {
	if s.Login.Secure() {
		return "__Host-account-handoff"
	}
	return "account-handoff-local"
}
func (s *SSO) cookie(w http.ResponseWriter, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: s.name(), Value: value, Path: "/", HttpOnly: true, Secure: s.Login.Secure(), SameSite: http.SameSiteStrictMode, MaxAge: age})
}
func (g *Gateway) sso(w http.ResponseWriter, r *http.Request) {
	s := g.SSO
	w.Header().Set("Referrer-Policy", "no-referrer")
	if strings.ToLower(r.Host) != s.Login.ExpectedHost() {
		fail(w, 400, "Некорректный адрес приложения")
		return
	}
	switch r.URL.Path {
	case "/api/account/authorize":
		http.SetCookie(w, &http.Cookie{Name: "fq-remember-choice", Value: r.URL.Query().Get("remember"), Path: "/", HttpOnly: true, Secure: s.Login.Secure(), SameSite: http.SameSiteLaxMode, MaxAge: 600})
		s.Login.Start(w, r)
	case "/api/account/callback":
		subject, err := s.Login.Complete(w, r)
		if err != nil {
			fail(w, 401, "Не удалось подтвердить единый вход. Вернитесь в приложение и повторите вход.")
			return
		}
		// The callback carries no application token or identity in the redirect URL.
		// Callback не передаёт токен приложения или идентификатор в URL перенаправления.
		var raw [32]byte
		if _, err = rand.Read(raw[:]); err != nil {
			fail(w, 503, "Вход временно недоступен")
			return
		}
		code := base64.RawURLEncoding.EncodeToString(raw[:])
		s.mu.Lock()
		for k, v := range s.pending {
			if !time.Now().Before(v.expires) {
				delete(s.pending, k)
			}
		}
		if len(s.pending) >= 10000 {
			s.mu.Unlock()
			fail(w, 503, "Вход временно недоступен")
			return
		}
		choice, _ := r.Cookie("fq-remember-choice")
		remember := choice != nil && choice.Value == "1"
		s.pending[code] = handoff{remember: remember, subject: subject, expires: time.Now().Add(time.Minute)}
		s.mu.Unlock()
		s.cookie(w, code, 60)
		http.Redirect(w, r, s.Login.ReturnURL(), http.StatusSeeOther)
	case "/api/account/exchange":
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		cookie, err := r.Cookie(s.name())
		if err != nil {
			fail(w, 401, "Повторите единый вход")
			return
		}
		s.cookie(w, "", -1)
		s.mu.Lock()
		h, ok := s.pending[cookie.Value]
		delete(s.pending, cookie.Value)
		s.mu.Unlock()
		if !ok || !time.Now().Before(h.expires) {
			fail(w, 401, "Повторите единый вход")
			return
		}
		family, owner, err := s.Catalog.Identity(r.Context(), h.subject.Issuer, h.subject.ID)
		if err != nil {
			if !errors.Is(err, domain.ErrUnauthorized) {
				fail(w, 503, "Проверка аккаунта временно недоступна")
				return
			}
			fail(w, 403, "Аккаунт не подключён к семье или доступ приостановлен. Обратитесь к оператору.")
			return
		}
		b, _ := json.Marshal(httpapi.AccountSessionInput{FamilyID: family, ParticipantID: owner})
		_, status, err := g.rpc(r, http.MethodPost, httpapi.AccountSessionPath, b)
		if err != nil || status != 200 {
			fail(w, 503, "Не удалось открыть семейный профиль. Повторите вход.")
			return
		}
		ttl := 12 * time.Hour
		age := 0
		if h.remember {
			ttl = 30 * 24 * time.Hour
			age = int(ttl.Seconds())
		}
		sessionID, err := s.Sessions.AddFor(h.subject, strconv.FormatInt(family, 10), ttl)
		if err != nil {
			fail(w, 401, "Повторите единый вход")
			return
		}
		if old, e := r.Cookie(s.sessionName()); e == nil {
			s.Sessions.Delete(old.Value)
		}
		s.sessionCookie(w, sessionID, age)
		// Clear the old device after Core has revoked it, as in password login.
		// Удаляем прежний cookie устройства после его отзыва Core, как при входе по паролю.
		http.SetCookie(w, &http.Cookie{Name: "familyquest_device", Path: "/api", HttpOnly: true, Secure: s.Login.Secure(), SameSite: http.SameSiteStrictMode, MaxAge: -1})
		g.accountProfiles(w, r, h.subject, family)
	case "/api/account/restore", "/api/account/profile":
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		c, e := r.Cookie(s.sessionName())
		if e != nil {
			fail(w, 401, "Войдите с единым аккаунтом")
			return
		}
		session, ok := s.Sessions.Get(c.Value)
		if !ok {
			fail(w, 401, "Войдите с единым аккаунтом")
			return
		}
		family, _, e := s.Catalog.Identity(r.Context(), session.Subject.Issuer, session.Subject.ID)
		if e != nil || strconv.FormatInt(family, 10) != session.Scope {
			fail(w, 401, "Доступ к семье завершён")
			return
		}
		if r.URL.Path == "/api/account/restore" {
			g.accountProfiles(w, r, session.Subject, family)
			return
		}
		if !g.limit("profile:" + session.Scope) {
			fail(w, 429, "Слишком много попыток. Повторите через минуту.")
			return
		}
		var input struct {
			ParticipantID int64  `json:"participantId"`
			PIN           string `json:"pin"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil || input.ParticipantID <= 0 {
			fail(w, 400, "Выберите профиль")
			return
		}
		b, _ := json.Marshal(input)
		g.accountCore(w, r, session.Subject, family, http.MethodPost, "/api/session", b)
	default:
		http.NotFound(w, r)
	}
}

func (s *SSO) sessionName() string {
	if s.Login.Secure() {
		return "__Host-familyquest-sso"
	}
	return "familyquest-sso-local"
}
func (s *SSO) sessionCookie(w http.ResponseWriter, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: s.sessionName(), Value: value, Path: "/", HttpOnly: true, Secure: s.Login.Secure(), SameSite: http.SameSiteLaxMode, MaxAge: age})
}
func (g *Gateway) backchannel(w http.ResponseWriter, r *http.Request) {
	sub, err := g.SSO.Login.VerifyLogout(r)
	if err != nil {
		fail(w, 400, "Invalid logout token")
		return
	}
	g.SSO.Sessions.Revoke(sub)
	w.WriteHeader(200)
}
func (g *Gateway) logoutSSO(w http.ResponseWriter, r *http.Request) {
	target, err := g.SSO.Login.LogoutURL()
	if err != nil {
		fail(w, 503, "Общий выход временно недоступен")
		return
	}
	if c, e := r.Cookie(g.SSO.sessionName()); e == nil {
		g.SSO.Sessions.Delete(c.Value)
	}
	g.SSO.sessionCookie(w, "", -1)
	_, _, _ = g.rpc(r, http.MethodPost, "/api/session/logout", []byte("{}"))
	http.SetCookie(w, &http.Cookie{Name: "familyquest_device", Path: "/api", HttpOnly: true, Secure: g.SSO.Login.Secure(), SameSite: http.SameSiteStrictMode, MaxAge: -1})
	jsonResponse(w, 200, map[string]string{"logoutUrl": target})
}

// Account cookies can only list profiles and verify a PIN; they never expose the owner's JWT.
// Cookie аккаунта разрешает только список профилей и проверку PIN, без выдачи JWT владельца.
func (g *Gateway) accountProfiles(w http.ResponseWriter, r *http.Request, sub identity.Subject, family int64) {
	g.accountCore(w, r, sub, family, http.MethodGet, "/api/participants", nil)
}
func (g *Gateway) accountCore(w http.ResponseWriter, r *http.Request, sub identity.Subject, family int64, method, path string, body []byte) {
	actual, owner, err := g.SSO.Catalog.Identity(r.Context(), sub.Issuer, sub.ID)
	if err != nil || actual != family {
		fail(w, 401, "Доступ к семье завершён")
		return
	}
	// Never forward a previous device: an account restore must require the profile PIN.
	// Старое устройство не передаём: восстановление аккаунта требует PIN профиля.
	internal := r.Clone(r.Context())
	internal.Header = r.Header.Clone()
	internal.Header.Del("Cookie")
	internal.Header.Del("Authorization")
	b, _ := json.Marshal(httpapi.AccountSessionInput{FamilyID: family, ParticipantID: owner})
	result, status, err := g.rpc(internal, http.MethodPost, httpapi.AccountSessionPath, b)
	var auth struct {
		Token string `json:"token"`
	}
	if err != nil || status != 200 || json.Unmarshal(result, &auth) != nil || auth.Token == "" {
		fail(w, 503, "Не удалось открыть семью")
		return
	}
	internal.Header.Set("Authorization", "Bearer "+auth.Token)
	result, status, err = g.rpc(internal, method, path, body)
	if err != nil {
		fail(w, 503, "Не удалось открыть профиль")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	io.Copy(w, strings.NewReader(string(result)))
}
