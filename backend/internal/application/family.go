package application

import (
	"context"
	"errors"
	"github.com/lobov/familyquest/backend/internal/domain"
	"strings"
	"time"
)

type FamilyRepository interface {
	ListFamilyEntries(context.Context) ([]domain.FamilyEntry, error)
	GetFamilyEntry(context.Context, int64) (domain.FamilyEntry, error)
	SaveFamilyEntry(context.Context, domain.FamilyEntry) (domain.FamilyEntry, error)
}

func (s *Service) FamilyEntries(ctx context.Context, p domain.Principal) ([]domain.FamilyEntry, error) {
	if !domain.IsFamilyMember(p) {
		return nil, domain.ErrForbidden
	}
	return s.repo.ListFamilyEntries(ctx)
}
func (s *Service) validateFamilyParticipants(ctx context.Context, e domain.FamilyEntry) error {
	if e.Kind == "sport" && e.Date > time.Now().UTC().Add(24*time.Hour).Format("2006-01-02") {
		return domain.ErrInvalidInput
	}
	if err := e.Validate(); err != nil {
		return err
	}
	ids := append([]int64{}, e.ParticipantIDs...)
	for _, step := range e.Steps {
		if step.ParticipantID > 0 {
			ids = append(ids, step.ParticipantID)
		}
	}
	for _, id := range ids {
		p, err := s.repo.GetParticipant(ctx, id)
		if err != nil {
			if errors.Is(err, domain.ErrUnauthorized) || errors.Is(err, domain.ErrNotFound) {
				return domain.ErrInvalidInput
			}
			return err
		}
		if !p.Active || !domain.IsFamilyMember(domain.Principal{ParticipantID: p.ID, Role: p.Role}) {
			return domain.ErrInvalidInput
		}
	}
	return nil
}
func (s *Service) CreateFamilyEntry(ctx context.Context, p domain.Principal, kind string, draft domain.FamilyDraft) (domain.FamilyEntry, error) {
	if !domain.IsFamilyMember(p) {
		return domain.FamilyEntry{}, domain.ErrForbidden
	}
	draft.Title = strings.TrimSpace(draft.Title)
	e := domain.FamilyEntry{Kind: kind, AuthorID: p.ParticipantID, FamilyDraft: draft, Events: []domain.FamilyEvent{}}
	// Children propose adventures; a parent agrees to the plan before execution.
	if !p.IsParent() && kind == "adventure" {
		e.Kind = "proposal"
	}
	if e.Kind == "sport" && !p.IsParent() && (len(e.ParticipantIDs) != 1 || e.ParticipantIDs[0] != p.ParticipantID) {
		return e, domain.ErrForbidden
	}
	if err := s.validateFamilyParticipants(ctx, e); err != nil {
		return e, err
	}
	return s.repo.SaveFamilyEntry(ctx, e)
}
func (s *Service) EditFamilyEntry(ctx context.Context, p domain.Principal, id int64, version int, draft domain.FamilyDraft) (domain.FamilyEntry, error) {
	if !domain.IsFamilyMember(p) {
		return domain.FamilyEntry{}, domain.ErrForbidden
	}
	e, err := s.repo.GetFamilyEntry(ctx, id)
	if err != nil {
		return e, err
	}
	if !e.CanEdit(p) {
		return e, domain.ErrForbidden
	}
	if e.Version != version || e.Archived {
		return e, domain.ErrConflict
	}
	// Once work starts, step identity/assignments are immutable so history stays valid.
	if len(e.Events) > 0 {
		if len(e.Steps) != len(draft.Steps) {
			return e, domain.ErrConflict
		}
		for i, step := range e.Steps {
			if step != draft.Steps[i] {
				return e, domain.ErrConflict
			}
		}
	}
	draft.Title = strings.TrimSpace(draft.Title)
	e.FamilyDraft = draft
	if e.Kind == "proposal" && !p.IsParent() {
		e.Approved = false
	}
	if e.Kind == "sport" && !p.IsParent() && (len(e.ParticipantIDs) != 1 || e.ParticipantIDs[0] != p.ParticipantID) {
		return e, domain.ErrForbidden
	}
	if err := s.validateFamilyParticipants(ctx, e); err != nil {
		return e, err
	}
	return s.repo.SaveFamilyEntry(ctx, e)
}
func (s *Service) FamilyAction(ctx context.Context, p domain.Principal, id int64, cmd domain.FamilyCommand) (domain.FamilyEntry, error) {
	if !domain.IsFamilyMember(p) {
		return domain.FamilyEntry{}, domain.ErrForbidden
	}
	e, err := s.repo.GetFamilyEntry(ctx, id)
	if err != nil {
		return e, err
	}
	if err = e.ApplyFamilyCommand(p, cmd, time.Now()); err != nil {
		return e, err
	}
	return s.repo.SaveFamilyEntry(ctx, e)
}
