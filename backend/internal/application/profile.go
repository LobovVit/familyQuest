package application

import (
	"context"
	"github.com/lobov/familyquest/backend/internal/domain"
)

type ProfileRepository interface {
	LearningProfile(context.Context, int64) (domain.LearningProfile, error)
	SaveLearningProfile(context.Context, int64, domain.LearningProfile) error
}

func (s *Service) Profile(ctx context.Context, p domain.Principal, id int64) (domain.LearningProfile, error) {
	if (s.familyID > 0 && p.FamilyID != s.familyID) || (!p.IsParent() && p.ParticipantID != id) {
		return domain.LearningProfile{}, domain.ErrForbidden
	}
	repo, ok := s.repo.(ProfileRepository)
	if !ok {
		return domain.LearningProfile{}, domain.ErrNotFound
	}
	return repo.LearningProfile(ctx, id)
}
func (s *Service) SaveProfile(ctx context.Context, p domain.Principal, id int64, v domain.LearningProfile) error {
	if err := s.writable(ctx); err != nil {
		return err
	}
	if !p.IsParent() || (s.familyID > 0 && p.FamilyID != s.familyID) {
		return domain.ErrForbidden
	}
	if e := v.Validate(s.now()); e != nil {
		return e
	}
	repo, ok := s.repo.(ProfileRepository)
	if !ok {
		return domain.ErrNotFound
	}
	return repo.SaveLearningProfile(ctx, id, v)
}
func (s *Service) Policy(ctx context.Context, p domain.Principal) (domain.LearningPolicy, error) {
	v, e := s.Profile(ctx, p, p.ParticipantID)
	return v.Policy(s.now()), e
}

func (s *Service) writable(ctx context.Context) error {
	if s.familyID == 0 {
		return nil
	}
	r, ok := s.repo.(interface{ CheckWriteAccess(context.Context) error })
	if !ok {
		return domain.ErrForbidden
	}
	return r.CheckWriteAccess(ctx)
}
