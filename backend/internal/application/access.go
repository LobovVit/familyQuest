package application

import (
	"context"

	"github.com/lobov/familyquest/backend/internal/domain"
)

// WriteAccessRepository is the subscription policy port for a family-bound service.
type WriteAccessRepository interface {
	CheckWriteAccess(context.Context) error
}

type mutation string

const (
	interactiveMutation mutation = "interactive"
	taskCompletion      mutation = "complete_task"
	taskConfirmation    mutation = "confirm_task"
	familyProgress      mutation = "family_progress"
)

// authorizeMutation enforces delegated permissions independently of the transport.
// A new use case is interactive unless it explicitly permits a delegated action.
func (s *Service) authorizeMutation(p domain.Principal, action mutation) error {
	if p.FamilyID != s.familyID || p.ParticipantID <= 0 || domain.ValidateRole(p.Role) != nil || p.ConfirmedUntil != 0 {
		return domain.ErrForbidden
	}
	if p.MCPGrantID == "" {
		return nil
	}
	if p.MCPReadOnly || !domain.IsFamilyMember(p) {
		return domain.ErrForbidden
	}
	switch action {
	case taskCompletion, taskConfirmation, familyProgress:
		return nil
	default:
		return domain.ErrForbidden
	}
}

func (s *Service) writable(ctx context.Context, p domain.Principal, action mutation) error {
	if err := s.authorizeMutation(p, action); err != nil {
		return err
	}
	if s.familyID == 0 {
		return nil
	}
	return s.repo.CheckWriteAccess(ctx)
}
