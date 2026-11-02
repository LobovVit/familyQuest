package application

import (
	"context"
	"strings"

	"github.com/lobov/familyquest/backend/internal/domain"
)

type NotificationRepository interface {
	SetNotificationRecipient(context.Context, domain.NotificationRecipient, bool) error
	ClaimNotification(context.Context) (*domain.NotificationDelivery, error)
	NotificationAllowed(context.Context, string, string) (bool, error)
	SettleNotification(context.Context, domain.NotificationSettlement) error
}

func validNotificationID(v string) bool {
	if len(v) != 36 {
		return false
	}
	for i, c := range v {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// Only the private Access/Core contract calls these methods; never a browser-provided identity.
func (s *Service) SetNotificationRecipient(ctx context.Context, v domain.NotificationRecipient, enabled bool) error {
	if s.familyID <= 0 || len(v.Reference) == 0 || len(v.Reference) > 128 || !validNotificationID(v.SubscriptionID) {
		return domain.ErrInvalidInput
	}
	if enabled {
		if len(v.Issuer) == 0 || len(v.Issuer) > 2048 || strings.TrimSpace(v.Subject) == "" || len(v.Subject) > 512 || v.ParentID <= 0 {
			return domain.ErrInvalidInput
		}
		p, e := s.repo.GetParticipant(ctx, v.ParentID)
		if e != nil {
			return e
		}
		if !p.Active || p.Role != domain.RoleParent {
			return domain.ErrForbidden
		}
	}
	return s.repo.SetNotificationRecipient(ctx, v, enabled)
}
func (s *Service) ClaimNotification(ctx context.Context) (*domain.NotificationDelivery, error) {
	if s.familyID <= 0 {
		return nil, domain.ErrForbidden
	}
	return s.repo.ClaimNotification(ctx)
}
func (s *Service) NotificationAllowed(ctx context.Context, id, lease string) (bool, error) {
	if s.familyID <= 0 || !validNotificationID(id) || !validNotificationID(lease) {
		return false, domain.ErrInvalidInput
	}
	return s.repo.NotificationAllowed(ctx, id, lease)
}
func (s *Service) SettleNotification(ctx context.Context, v domain.NotificationSettlement) error {
	if s.familyID <= 0 || !validNotificationID(v.ID) || !validNotificationID(v.Lease) || len(v.PortalID) > 128 {
		return domain.ErrInvalidInput
	}
	switch v.Outcome {
	case "delivered":
		if !strings.HasPrefix(v.PortalID, "tgn_") {
			return domain.ErrInvalidInput
		}
	case "retry", "cancelled", "failed":
	default:
		return domain.ErrInvalidInput
	}
	switch v.ErrorCode {
	case "", "transport", "portal_unavailable", "portal_denied", "portal_conflict", "portal_invalid", "identity_unavailable", "registry_unavailable", "recipient_revoked", "parent_revoked", "invalid_response":
	default:
		return domain.ErrInvalidInput
	}
	return s.repo.SettleNotification(ctx, v)
}
