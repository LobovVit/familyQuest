package access

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/lobov/familyquest/backend/internal/domain"
	"github.com/lobov/familyquest/backend/internal/httpapi"
)

// The single Access process serializes consent and dispatch. Durable recipient state,
// generations and revocations belong to Core PostgreSQL, never an Access file.
type TelegramNotifications struct{ deliveryMu sync.Mutex }

func NewTelegramNotifications() *TelegramNotifications { return &TelegramNotifications{} }

type telegramBinding struct {
	BindingID    string `json:"bindingId"`
	RecipientRef string `json:"recipientRef"`
	Issuer       string `json:"issuer"`
	Subject      string `json:"subject"`
}

func notificationBinding(v domain.NotificationRecipient) telegramBinding {
	return telegramBinding{v.BindingID, v.Reference, v.Issuer, v.Subject}
}
func (b telegramBinding) matches(s telegramSession) bool {
	return strings.HasPrefix(b.BindingID, "tgb_") && len(b.BindingID) > 4 && len(b.BindingID) <= 128 && b.RecipientRef == s.RecipientRef && b.Issuer == s.Issuer && b.Subject == s.Subject
}
func newSubscriptionID() (string, error) {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", e
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
func (g *Gateway) telegramConsent(w http.ResponseWriter, r *http.Request, s telegramSession) {
	var in struct {
		Enabled bool `json:"enabled"`
	}
	if !telegramInput(w, r, &in) {
		return
	}
	n := g.Telegram.Notifications
	if n == nil {
		fail(w, 503, "Уведомления пока не настроены")
		return
	}
	n.deliveryMu.Lock()
	defer n.deliveryMu.Unlock()
	v := domain.NotificationRecipient{Reference: s.RecipientRef, Issuer: s.Issuer, Subject: s.Subject}
	if in.Enabled {
		p, e := g.Telegram.validate(r.Context(), s)
		if e != nil {
			telegramFail(w, e)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			fail(w, 401, "Войдите в родительский профиль с PIN")
			return
		}
		if !g.telegramFamily(w, r, s, p) {
			return
		}
		b, status, e := g.rpc(r, http.MethodGet, httpapi.IntrospectionPath, nil)
		var state httpapi.AccessState
		if e != nil || status != 200 || json.Unmarshal(b, &state) != nil {
			fail(w, 401, "Войдите в родительский профиль")
			return
		}
		if !state.Principal.IsParent() || state.Principal.FamilyID != s.Family || state.Subscription.Status != "active" || !state.Subscription.Access {
			fail(w, 403, "Уведомления доступны только взрослому своей семьи с действующим доступом")
			return
		}
		v.ParentID = state.Principal.ParticipantID
		v.SubscriptionID, e = newSubscriptionID()
		if e != nil {
			telegramFail(w, e)
			return
		}
	}
	// A selected family is server-validated. Before selection, remote opt-out still
	// revokes all bindings for this bot; old local rows are cancelled by eligibility.
	var localErr error
	if !in.Enabled && s.Family > 0 {
		localErr = g.notificationRPC(r.Context(), "revoke", httpapi.NotificationRequest{FamilyID: s.Family, Recipient: domain.NotificationRecipient{Reference: s.RecipientRef}}, nil)
	}
	remoteErr := g.Telegram.Portal.Call(r.Context(), "notifications/consent", map[string]any{"sessionToken": s.Token, "enabled": in.Enabled}, nil)
	if remoteErr != nil {
		telegramFail(w, remoteErr)
		return
	}
	if localErr != nil {
		telegramFail(w, localErr)
		return
	}
	if in.Enabled {
		var binding telegramBinding
		e := g.Telegram.Portal.Call(r.Context(), "notifications/binding", map[string]string{"sessionToken": s.Token}, &binding)
		if e == nil && !binding.matches(s) {
			e = errors.New("invalid notification binding")
		}
		if e == nil {
			v.BindingID = binding.BindingID
			e = g.notificationRPC(r.Context(), "recipient", httpapi.NotificationRequest{FamilyID: s.Family, Recipient: v, Enabled: true}, nil)
		}
		if e != nil {
			// No new SQL generation is enabled without the verified binding. Existing
			// consent is never implicitly migrated to another identity or parent.
			_ = g.Telegram.Portal.Call(r.Context(), "notifications/consent", map[string]any{"sessionToken": s.Token, "enabled": false}, nil)
			telegramFail(w, e)
			return
		}
	}
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
