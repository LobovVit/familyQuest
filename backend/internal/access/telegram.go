package access

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lobov/familyquest/backend/internal/domain"
	"github.com/lobov/familyquest/backend/internal/httpapi"
	"github.com/lobov/familyquest/backend/internal/identity"
)

const telegramCookie = "familyquest_telegram"

type telegramSession struct {
	Token           string
	RecipientRef    string
	Expires         time.Time
	Family          int64
	Issuer, Subject string
}
type Telegram struct {
	Portal             TelegramPortal
	Catalog            IdentityCatalog
	Issuer, AccountURL string
	Secure             bool
	Notifications      *TelegramNotifications
	mu                 sync.Mutex
	sessions           map[string]telegramSession
}

func NewTelegram(portal TelegramPortal, catalog IdentityCatalog, issuer, accountURL string, secure bool) *Telegram {
	return &Telegram{Portal: portal, Catalog: catalog, Issuer: issuer, AccountURL: accountURL, Secure: secure, sessions: map[string]telegramSession{}}
}
func (t *Telegram) cookie(w http.ResponseWriter, id string, age int) {
	sameSite := http.SameSiteStrictMode
	if t.Secure {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{Name: telegramCookie, Value: id, Path: "/api", HttpOnly: true, Secure: t.Secure, SameSite: sameSite, Partitioned: t.Secure, MaxAge: age})
}
func (t *Telegram) get(r *http.Request) (string, telegramSession, bool) {
	c, e := r.Cookie(telegramCookie)
	if e != nil {
		return "", telegramSession{}, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	s, ok := t.sessions[c.Value]
	if !ok || !time.Now().Before(s.Expires) {
		delete(t.sessions, c.Value)
		return "", telegramSession{}, false
	}
	return c.Value, s, true
}
func (t *Telegram) remove(id string) { t.mu.Lock(); defer t.mu.Unlock(); delete(t.sessions, id) }
func (t *Telegram) replace(w http.ResponseWriter, r *http.Request, s telegramSession, rotate bool) error {
	var raw [32]byte
	if _, e := rand.Read(raw[:]); e != nil {
		return e
	}
	id := base64.RawURLEncoding.EncodeToString(raw[:])
	t.mu.Lock()
	defer t.mu.Unlock()
	for k, v := range t.sessions {
		if !time.Now().Before(v.Expires) {
			delete(t.sessions, k)
		}
	}
	if len(t.sessions) >= 10000 {
		return errors.New("session capacity")
	}
	if rotate {
		c, e := r.Cookie(telegramCookie)
		if e != nil {
			return telegramPortalError{status: 401}
		}
		if _, ok := t.sessions[c.Value]; !ok {
			return telegramPortalError{status: 401}
		}
	}
	if c, e := r.Cookie(telegramCookie); e == nil {
		delete(t.sessions, c.Value)
	}
	t.sessions[id] = s
	t.cookie(w, id, int(time.Until(s.Expires).Seconds()))
	return nil
}
func (t *Telegram) validate(ctx context.Context, s telegramSession) (TelegramPrincipal, error) {
	var out telegramExchange
	if e := t.Portal.Call(ctx, "session", map[string]string{"sessionToken": s.Token}, &out); e != nil {
		return TelegramPrincipal{}, e
	}
	p := out.Principal
	if p.ProductID != "familyquest" || p.BotID != "familyquest" || !time.Now().Before(out.ExpiresAt) || out.ExpiresAt.After(s.Expires.Add(time.Second)) || p.RecipientRef == "" || (s.RecipientRef != "" && s.RecipientRef != p.RecipientRef) ||
		(p.IdentityState != "linked" && p.IdentityState != "unlinked") ||
		(p.IdentityState == "linked" && (p.Issuer != t.Issuer || p.Subject == "")) ||
		(s.Subject != "" && (p.IdentityState != "linked" || p.Subject != s.Subject || p.Issuer != s.Issuer)) {
		return p, telegramPortalError{status: 401}
	}
	return p, nil
}
func telegramFail(w http.ResponseWriter, e error) {
	var pe telegramPortalError
	if errors.As(e, &pe) && (pe.status == 401 || pe.status == 403 || pe.status == 409) {
		message := "Сессия Telegram завершена или доступ отозван. Откройте приложение заново."
		if pe.status == 403 {
			message = "Действие недоступно. Проверьте привязку аккаунта и разрешение боту писать; после нового разрешения заново откройте Mini App."
		}
		if pe.status == 409 {
			message = "Запрос уже использован или возник конфликт привязки. Проверьте связь в Делодне; для нового входа заново откройте Mini App."
		}
		fail(w, pe.status, message)
		return
	}
	fail(w, 503, "Telegram-вход временно недоступен. Повторите позже или используйте обычный вход.")
}
func telegramInput(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		fail(w, 400, "Проверьте данные запроса")
		return false
	}
	return true
}
func (g *Gateway) telegram(w http.ResponseWriter, r *http.Request) {
	t := g.Telegram
	if t == nil {
		fail(w, 503, "Telegram-вход пока не включён. Используйте обычный вход.")
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	if r.URL.Path == "/api/telegram/exchange" {
		var in struct {
			InitData string `json:"initData"`
		}
		if !telegramInput(w, r, &in) {
			return
		}
		if in.InitData == "" {
			fail(w, 400, "Откройте приложение из Telegram")
			return
		}
		var out telegramExchange
		if e := t.Portal.Call(r.Context(), "exchange", map[string]string{"botId": "familyquest", "initData": in.InitData}, &out); e != nil {
			telegramFail(w, e)
			return
		}
		if out.SessionToken == "" || out.Principal.ProductID != "familyquest" || out.Principal.BotID != "familyquest" || !time.Now().Before(out.ExpiresAt) || out.ExpiresAt.After(time.Now().Add(12*time.Hour+time.Minute)) {
			telegramFail(w, errors.New("invalid exchange"))
			return
		}
		s := telegramSession{Token: out.SessionToken, Expires: out.ExpiresAt}
		p, e := t.validate(r.Context(), s)
		if e != nil {
			telegramFail(w, e)
			return
		}
		s.RecipientRef = p.RecipientRef
		if e = t.replace(w, r, s, false); e != nil {
			telegramFail(w, e)
			return
		}

		g.telegramState(w, r, s, p)
		return
	}
	id, s, ok := t.get(r)
	if !ok {
		fail(w, 401, "Откройте приложение заново из Telegram")
		return
	}
	if r.URL.Path == "/api/telegram/logout" {
		t.remove(id)
		t.cookie(w, "", -1)
		// Local revocation is effective even during a Portal outage.
		_ = t.Portal.Call(r.Context(), "logout", map[string]string{"sessionToken": s.Token}, nil)
		jsonResponse(w, 200, map[string]bool{"ok": true})
		return
	}
	if r.URL.Path == "/api/telegram/consent" {
		g.telegramConsent(w, r, s)
		return
	}
	p, e := t.validate(r.Context(), s)
	if e != nil {
		telegramFail(w, e)
		return
	}
	switch r.URL.Path {
	case "/api/telegram/session":
		g.telegramState(w, r, s, p)
	case "/api/telegram/link":
		var in struct {
			LinkCode string `json:"linkCode"`
		}
		if !telegramInput(w, r, &in) {
			return
		}
		if len(in.LinkCode) < 8 || len(in.LinkCode) > 512 {
			fail(w, 400, "Введите код привязки из Делодня")
			return
		}
		if e = t.Portal.Call(r.Context(), "link/claim", map[string]string{"sessionToken": s.Token, "linkCode": in.LinkCode}, nil); e != nil {
			telegramFail(w, e)
			return
		}
		jsonResponse(w, 200, map[string]bool{"ok": true})
	case "/api/telegram/family":
		var in struct {
			FamilyID int64 `json:"familyId"`
		}
		if !telegramInput(w, r, &in) {
			return
		}
		if in.FamilyID == 0 {
			s.Family = 0
			s.Subject = ""
			s.Issuer = ""
			if e = t.replace(w, r, s, true); e != nil {
				telegramFail(w, e)
				return
			}
			g.telegramState(w, r, s, p)
			return
		}
		if p.IdentityState != "linked" {
			fail(w, 403, "Сначала привяжите существующий аккаунт")
			return
		}
		family, _, err := t.Catalog.Identity(r.Context(), p.Issuer, p.Subject)
		if err != nil || family != in.FamilyID {
			fail(w, 403, "Семья недоступна")
			return
		}
		s.Family = family
		s.Issuer = p.Issuer
		s.Subject = p.Subject
		if e = t.replace(w, r, s, true); e != nil {
			telegramFail(w, e)
			return
		}
		g.accountProfiles(w, r, identity.Subject{Issuer: p.Issuer, ID: p.Subject}, family)
	case "/api/telegram/profile":
		if !g.telegramFamily(w, r, s, p) {
			return
		}
		if !g.limit("profile:" + strconv.FormatInt(s.Family, 10)) {
			fail(w, 429, "Слишком много попыток. Повторите через минуту.")
			return
		}
		var in struct {
			ParticipantID int64  `json:"participantId"`
			PIN           string `json:"pin"`
		}
		if !telegramInput(w, r, &in) {
			return
		}
		if in.ParticipantID <= 0 || domain.ValidatePIN(in.PIN) != nil {
			fail(w, 400, "Выберите профиль и введите PIN")
			return
		}
		b, _ := json.Marshal(in)
		g.accountCore(w, r, identity.Subject{Issuer: p.Issuer, ID: p.Subject}, s.Family, http.MethodPost, "/api/session", b)
	default:
		http.NotFound(w, r)
	}
}
func (g *Gateway) telegramFamily(w http.ResponseWriter, r *http.Request, s telegramSession, p TelegramPrincipal) bool {
	if p.IdentityState != "linked" || s.Family <= 0 || s.Subject != p.Subject || s.Issuer != p.Issuer {
		fail(w, 403, "Выберите свою семью")
		return false
	}
	family, _, e := g.Telegram.Catalog.Identity(r.Context(), p.Issuer, p.Subject)
	if e != nil || family != s.Family {
		fail(w, 403, "Доступ к семье завершён")
		return false
	}
	return true
}
func (g *Gateway) telegramState(w http.ResponseWriter, r *http.Request, s telegramSession, p TelegramPrincipal) {
	families := []map[string]any{}
	if p.IdentityState == "linked" {
		family, _, e := g.Telegram.Catalog.Identity(r.Context(), p.Issuer, p.Subject)
		if e != nil && !errors.Is(e, domain.ErrUnauthorized) {
			telegramFail(w, e)
			return
		}
		if e == nil {
			internal := r.Clone(r.Context())
			internal.Header = make(http.Header)
			b, _ := json.Marshal(map[string]int64{"familyId": family})
			body, status, e := g.rpc(internal, http.MethodPost, httpapi.SubscriptionSummaryPath, b)
			var sub domain.FamilySubscription
			if e != nil || status != 200 || json.Unmarshal(body, &sub) != nil || sub.FamilyID != family {
				telegramFail(w, errors.New("family unavailable"))
				return
			}
			families = append(families, map[string]any{"id": family, "name": sub.Name})
		}
	}
	jsonResponse(w, 200, map[string]any{"linked": p.IdentityState == "linked", "families": families, "accountUrl": g.Telegram.AccountURL, "expiresAt": s.Expires})
}

// A Telegram cookie is independently checked on every protected request; there is no OIDC fallback.
func (g *Gateway) telegramAccess(w http.ResponseWriter, r *http.Request, state httpapi.AccessState) bool {
	if g.Telegram == nil {
		fail(w, 401, "Telegram-вход отключён")
		return false
	}
	_, s, ok := g.Telegram.get(r)
	if !ok {
		fail(w, 401, "Сессия Telegram завершена")
		return false
	}
	p, e := g.Telegram.validate(r.Context(), s)
	if e != nil {
		telegramFail(w, e)
		return false
	}
	if !g.telegramFamily(w, r, s, p) {
		return false
	}
	if s.Family != state.Principal.FamilyID {
		fail(w, 403, "Выбранная семья не совпадает с профилем")
		return false
	}
	return true
}
func isTelegramRequest(r *http.Request) bool {
	return r.Header.Get("X-FamilyQuest-Client") == "telegram" || strings.HasPrefix(r.URL.Path, "/api/telegram/")
}
