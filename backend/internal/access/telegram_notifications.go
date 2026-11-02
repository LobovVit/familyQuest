package access

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lobov/familyquest/backend/internal/domain"
	"github.com/lobov/familyquest/backend/internal/httpapi"
)

type telegramRecipient struct {
	Disabled             bool
	Ref, Issuer, Subject string
	SubscriptionID       string
	Family, Parent       int64
}

// This registry stores adult opt-ins and revocation tombstones. Core owns the transactional
// handoff outbox; Portal owns remote consent, deduplication and Telegram delivery.
// Sessions and initData are deliberately never persisted here.
type TelegramNotifications struct {
	mu         sync.Mutex
	deliveryMu sync.Mutex
	path       string
	recipients map[string]telegramRecipient
}

func NewTelegramNotifications(path string) (*TelegramNotifications, error) {
	n := &TelegramNotifications{path: path, recipients: map[string]telegramRecipient{}}
	if path == "" {
		return n, nil
	} // Explicit in-memory adapter for tests.
	b, e := os.ReadFile(path)
	if e != nil && !os.IsNotExist(e) {
		return nil, e
	}
	if e == nil {
		if e = json.Unmarshal(b, &n.recipients); e != nil {
			return nil, e
		}
		if n.recipients == nil {
			n.recipients = map[string]telegramRecipient{}
		}
	}
	if len(n.recipients) > 10000 {
		return nil, errors.New("too many notification recipients")
	}
	return n, nil
}
func (n *TelegramNotifications) set(v telegramRecipient, enabled bool) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	next := make(map[string]telegramRecipient, len(n.recipients))
	for k, v := range n.recipients {
		next[k] = v
	}
	if _, exists := next[v.Ref]; !exists && len(next) >= 10000 {
		return errors.New("recipient capacity")
	}
	if enabled {
		v.Disabled = false
		next[v.Ref] = v
	} else {
		if previous, ok := next[v.Ref]; ok {
			v = previous
		}
		v.Disabled = true
		next[v.Ref] = v
	}
	if n.path != "" {
		b, e := json.Marshal(next)
		if e != nil {
			return e
		}
		f, e := os.CreateTemp(filepath.Dir(n.path), ".telegram-recipients-")
		if e != nil {
			return e
		}
		defer os.Remove(f.Name())
		if _, e = f.Write(b); e != nil {
			f.Close()
			return e
		}
		if e = f.Sync(); e != nil {
			f.Close()
			return e
		}
		if e = f.Close(); e != nil {
			return e
		}
		if e = os.Rename(f.Name(), n.path); e != nil {
			return e
		}
		dir, e := os.Open(filepath.Dir(n.path))
		if e != nil {
			return e
		}
		e = dir.Sync()
		closeErr := dir.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
	}
	n.recipients = next
	return nil
}
func (n *TelegramNotifications) family(id int64) []telegramRecipient {
	n.mu.Lock()
	defer n.mu.Unlock()
	result := []telegramRecipient{}
	for _, v := range n.recipients {
		if v.Family == id && !v.Disabled {
			result = append(result, v)
		}
	}
	return result
}
func (n *TelegramNotifications) get(ref string) (telegramRecipient, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	v, ok := n.recipients[ref]
	return v, ok
}
func (v telegramRecipient) domain() domain.NotificationRecipient {
	return domain.NotificationRecipient{Reference: v.Ref, SubscriptionID: v.SubscriptionID, Issuer: v.Issuer, Subject: v.Subject, ParentID: v.Parent}
}
func newSubscriptionID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
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
	v := telegramRecipient{Ref: s.RecipientRef, Issuer: s.Issuer, Subject: s.Subject, Family: s.Family}
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
		if !state.Principal.IsParent() || state.Principal.FamilyID != s.Family || state.Subscription.Status != "active" {
			fail(w, 403, "Уведомления доступны только взрослому своей семьи")
			return
		}
		v.Parent = state.Principal.ParticipantID
		v.SubscriptionID, e = newSubscriptionID()
		if e != nil {
			telegramFail(w, e)
			return
		}
	}
	old, exists := n.get(v.Ref)
	// Durable local revocation precedes both remote calls, including during a Portal outage.
	if e := n.set(v, false); e != nil {
		telegramFail(w, e)
		return
	}
	if exists && old.SubscriptionID != "" {
		if e := g.notificationRPC(r.Context(), "recipient", httpapi.NotificationRequest{FamilyID: old.Family, Recipient: old.domain(), Enabled: false}, nil); e != nil && in.Enabled {
			telegramFail(w, e)
			return
		}
	}
	if e := g.Telegram.Portal.Call(r.Context(), "notifications/consent", map[string]any{"sessionToken": s.Token, "enabled": in.Enabled}, nil); e != nil {
		telegramFail(w, e)
		return
	}
	if in.Enabled {
		e := g.notificationRPC(r.Context(), "recipient", httpapi.NotificationRequest{FamilyID: v.Family, Recipient: v.domain(), Enabled: true}, nil)
		if e == nil {
			e = n.set(v, true)
		}
		if e != nil {
			_ = g.Telegram.Portal.Call(r.Context(), "notifications/consent", map[string]any{"sessionToken": s.Token, "enabled": false}, nil)
			telegramFail(w, e)
			return
		}
	}
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
