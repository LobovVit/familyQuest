package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/lobov/familyquest/backend/internal/domain"
	"time"
)

type LearningRepository interface {
	FinishMath(context.Context, int64, string) (domain.MathSession, error)
	CreateMathSession(context.Context, domain.MathSession) (domain.MathSession, error)
	ListMathSessions(context.Context, int64) ([]domain.MathSession, error)
	AnswerMath(context.Context, int64, string, int, map[string]int, time.Time) (domain.MathSession, error)
	ActivityRewards(context.Context, int64) ([]domain.ActivityReward, error)
}

func (s *Service) StartMath(ctx context.Context, p domain.Principal, settings domain.MathSettings) (domain.MathView, error) {
	if p.ParticipantID <= 0 || p.Role != domain.RoleChild {
		return domain.MathView{}, domain.ErrForbidden
	}
	if err := settings.Validate(); err != nil {
		return domain.MathView{}, err
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return domain.MathView{}, err
	}
	saved, err := s.repo.CreateMathSession(ctx, domain.NewMathSession(hex.EncodeToString(id[:]), p.ParticipantID, settings, time.Now()))
	return saved.View(), err
}
func (s *Service) MathSessions(ctx context.Context, p domain.Principal) ([]domain.MathView, error) {
	if p.ParticipantID <= 0 || p.Role != domain.RoleChild {
		return nil, domain.ErrForbidden
	}
	sessions, err := s.repo.ListMathSessions(ctx, p.ParticipantID)
	if err != nil {
		return nil, err
	}
	out := []domain.MathView{}
	for _, session := range sessions {
		out = append(out, session.View())
	}
	return out, nil
}
func (s *Service) AnswerMath(ctx context.Context, p domain.Principal, id string, index int, values map[string]int) (domain.MathView, error) {
	if p.ParticipantID <= 0 || p.Role != domain.RoleChild {
		return domain.MathView{}, domain.ErrForbidden
	}
	session, err := s.repo.AnswerMath(ctx, p.ParticipantID, id, index, values, time.Now())
	return session.View(), err
}
func (s *Service) ActivityRewards(ctx context.Context, p domain.Principal) ([]domain.ActivityReward, error) {
	if !domain.IsFamilyMember(p) {
		return nil, domain.ErrForbidden
	}
	return s.repo.ActivityRewards(ctx, p.ParticipantID)
}

func (s *Service) FinishMath(ctx context.Context, p domain.Principal, id string) (domain.MathView, error) {
	if p.ParticipantID <= 0 || p.Role != domain.RoleChild {
		return domain.MathView{}, domain.ErrForbidden
	}
	session, err := s.repo.FinishMath(ctx, p.ParticipantID, id)
	return session.View(), err
}
