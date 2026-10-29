package access

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/lobov/familyquest/backend/internal/domain"
	"github.com/lobov/familyquest/backend/internal/identity"
)

type FamilyRegistrar interface {
	RegisterFamily(context.Context, string, string, domain.FamilyRegistration) (int64, error)
}

const onboardingScope = "onboarding"

func (g *Gateway) unbound(w http.ResponseWriter, r *http.Request, sub identity.Subject) bool {
	_, err := g.SSO.Catalog.PortalFamily(r.Context(), sub.Issuer, sub.ID)
	if errors.Is(err, domain.ErrUnauthorized) {
		return true
	}
	if err != nil {
		fail(w, 503, "Проверка аккаунта временно недоступна")
	} else {
		fail(w, 403, "Доступ к семье приостановлен. Обратитесь к оператору.")
	}
	return false
}
func (g *Gateway) beginOnboarding(w http.ResponseWriter, r *http.Request, sub identity.Subject) {
	if !g.unbound(w, r, sub) {
		return
	}
	s := g.SSO
	id, err := s.Sessions.AddFor(sub, onboardingScope, 30*time.Minute)
	if err != nil {
		fail(w, 401, "Повторите единый вход")
		return
	}
	if old, err := r.Cookie(s.sessionName()); err == nil {
		s.Sessions.Delete(old.Value)
	}
	s.sessionCookie(w, id, 1800)
	jsonResponse(w, 200, map[string]bool{"onboarding": true})
}
func (g *Gateway) onboarding(w http.ResponseWriter, r *http.Request, session identity.Session) {
	if g.Registrar == nil {
		fail(w, 503, "Создание семьи временно недоступно")
		return
	}
	if r.URL.Path == "/api/account/profile" {
		fail(w, 403, "Сначала создайте семью")
		return
	}
	sub := session.Subject
	family, owner, err := g.SSO.Catalog.Identity(r.Context(), sub.Issuer, sub.ID)
	if err == nil {
		// Recover after a lost creation response or creation in another browser tab.
		g.enterFamily(w, r, sub, family, owner)
		return
	}
	if !errors.Is(err, domain.ErrUnauthorized) {
		fail(w, 503, "Проверка аккаунта временно недоступна")
		return
	}
	if !g.unbound(w, r, sub) {
		return
	}
	if r.URL.Path == "/api/account/restore" {
		jsonResponse(w, 200, map[string]bool{"onboarding": true})
		return
	}
	if !g.limit("register:" + sub.Issuer + ":" + sub.ID) {
		fail(w, 429, "Слишком много попыток. Повторите через минуту.")
		return
	}
	var in domain.FamilyRegistration
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil || decoder.Decode(new(any)) != io.EOF {
		fail(w, 400, "Проверьте название семьи, имя и PIN")
		return
	}
	in, err = in.Normalize()
	if err != nil {
		fail(w, 400, "Название и имя: от 1 до 80 символов. PIN: 6 цифр.")
		return
	}
	family, err = g.Registrar.RegisterFamily(r.Context(), sub.Issuer, sub.ID, in)
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		fail(w, 400, "Проверьте название семьи, имя и PIN")
		return
	case errors.Is(err, domain.ErrForbidden), errors.Is(err, domain.ErrConflict):
		fail(w, 403, "Доступ к семье приостановлен. Обратитесь к оператору.")
		return
	case err != nil:
		fail(w, 503, "Не удалось создать семью. Повторите попытку — вторая семья не появится.")
		return
	}
	// A logout or revocation during provisioning must not establish a new session.
	cookie, err := r.Cookie(g.SSO.sessionName())
	if err != nil {
		fail(w, 401, "Повторите единый вход")
		return
	}
	if _, ok := g.SSO.Sessions.Get(cookie.Value); !ok {
		fail(w, 401, "Повторите единый вход")
		return
	}
	actual, owner, err := g.SSO.Catalog.Identity(r.Context(), sub.Issuer, sub.ID)
	if err != nil || actual != family {
		fail(w, 503, "Семья создана. Повторите вход, чтобы открыть профили.")
		return
	}
	g.enterFamily(w, r, sub, family, owner)
}
