package httpapi

import (
	"net/http"

	"github.com/lobov/familyquest/backend/internal/domain"
)

const NotificationPath = "/api/__access/notifications/"

type NotificationRequest struct {
	FamilyID   int64                         `json:"familyId"`
	Recipient  domain.NotificationRecipient  `json:"recipient,omitempty"`
	Enabled    bool                          `json:"enabled,omitempty"`
	Settlement domain.NotificationSettlement `json:"settlement,omitempty"`
	ID         string                        `json:"id,omitempty"`
	Lease      string                        `json:"lease,omitempty"`
}

func (s *SaaS) notifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	var in NotificationRequest
	if decodeJSON(r, &in) != nil || in.FamilyID <= 0 {
		respond(w, nil, domain.ErrInvalidInput)
		return
	}
	app, e := s.resolve(r.Context(), in.FamilyID)
	if e != nil {
		respond(w, nil, e)
		return
	}
	switch r.URL.Path {
	case NotificationPath + "recipient":
		respond(w, map[string]bool{"ok": true}, app.SetNotificationRecipient(r.Context(), in.Recipient, in.Enabled))
	case NotificationPath + "claim":
		v, e := app.ClaimNotification(r.Context())
		respond(w, v, e)
	case NotificationPath + "check":
		v, e := app.NotificationAllowed(r.Context(), in.ID, in.Lease)
		respond(w, map[string]bool{"allowed": v}, e)
	case NotificationPath + "settle":
		respond(w, map[string]bool{"ok": true}, app.SettleNotification(r.Context(), in.Settlement))
	default:
		http.NotFound(w, r)
	}
}
