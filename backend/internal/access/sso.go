package access

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/lobov/familyquest/backend/internal/domain"
	"net/http"
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
	subject identity.Subject
	expires time.Time
}
type SSO struct {
	Login   *identity.Login
	Catalog IdentityCatalog
	mu      sync.Mutex
	pending map[string]handoff
}

func NewSSO(login *identity.Login, catalog IdentityCatalog) *SSO {
	return &SSO{Login: login, Catalog: catalog, pending: map[string]handoff{}}
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
		s.pending[code] = handoff{subject: subject, expires: time.Now().Add(time.Minute)}
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
		response, status, err := g.rpc(r, http.MethodPost, httpapi.AccountSessionPath, b)
		if err != nil || status != 200 {
			fail(w, 503, "Не удалось открыть семейный профиль. Повторите вход.")
			return
		}
		// Clear the old device after Core has revoked it, as in password login.
		// Удаляем прежний cookie устройства после его отзыва Core, как при входе по паролю.
		http.SetCookie(w, &http.Cookie{Name: "familyquest_device", Path: "/api", HttpOnly: true, Secure: s.Login.Secure(), SameSite: http.SameSiteStrictMode, MaxAge: -1})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(response)
	default:
		http.NotFound(w, r)
	}
}
