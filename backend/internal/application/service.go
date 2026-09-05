package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/lobov/familyquest/backend/internal/auth"
	"github.com/lobov/familyquest/backend/internal/domain"
)

// Repository is the persistence port consumed by application use cases.
type Repository interface {
	Ping(context.Context) error
	ListParticipants(context.Context) ([]domain.Participant, error)
	CreateParticipant(context.Context, domain.Participant, string) (domain.Participant, error)
	DeleteParticipant(context.Context, int64) error
	UpdateParticipantPIN(context.Context, int64, string) (domain.Participant, error)
	VerifyParticipantPIN(context.Context, int64, string) (domain.Participant, error)
	ListChores(context.Context) ([]domain.Chore, error)
	CreateChore(context.Context, domain.Chore) (domain.Chore, error)
	UpdateChore(context.Context, domain.Chore) (domain.Chore, error)
	ListAssignments(context.Context) ([]domain.Assignment, error)
	CreateAssignment(context.Context, int64, int64) (domain.Assignment, error)
	ListTasks(context.Context, time.Time) ([]domain.Task, error)
	ListWeekPlan(context.Context, time.Time) ([]domain.WeekPlanItem, error)
	TaskOwner(context.Context, int64) (int64, error)
	CompleteTask(context.Context, int64, int64) (domain.Task, error)
	ConfirmTask(context.Context, int64, int64, int, string) (domain.Task, error)
	Leaderboard(context.Context, string, time.Time) ([]domain.LeaderboardEntry, error)
	ListBehaviorRatings(context.Context, time.Time) ([]domain.BehaviorRating, error)
	RateBehavior(context.Context, time.Time, int64, int64, int, string) (domain.BehaviorRating, error)
	ListRewards(context.Context) ([]domain.Reward, error)
	CreateReward(context.Context, domain.Reward) (domain.Reward, error)
	DeleteReward(context.Context, int64) error
	ExportBackup(context.Context) (BackupData, error)
	ImportBackup(context.Context, BackupData) error
}

type Service struct {
	repo   Repository
	tokens *auth.Tokens
}

func New(repo Repository, tokens *auth.Tokens) *Service { return &Service{repo: repo, tokens: tokens} }
func (s *Service) Ready(c context.Context) error        { return s.repo.Ping(c) }
func (s *Service) Authenticate(c context.Context, id int64, pin string) (domain.Participant, string, error) {
	if err := domain.ValidatePIN(pin); err != nil {
		return domain.Participant{}, "", err
	}
	p, err := s.repo.VerifyParticipantPIN(c, id, pin)
	if err != nil {
		return p, "", err
	}
	t, err := s.tokens.Issue(p)
	return p, t, err
}
func (s *Service) ParseToken(v string) (domain.Principal, error) {
	t, e := auth.Bearer(v)
	if e != nil {
		return domain.Principal{}, e
	}
	return s.tokens.Parse(t)
}
func (s *Service) ListParticipants(c context.Context) ([]domain.Participant, error) {
	return s.repo.ListParticipants(c)
}
func (s *Service) CreateParticipant(c context.Context, p domain.Participant, pin string) (domain.Participant, error) {
	if e := domain.ValidatePIN(pin); e != nil {
		return p, e
	}
	if p.Role == "" {
		p.Role = domain.RoleChild
	}
	if e := domain.ValidateRole(p.Role); e != nil {
		return p, e
	}
	return s.repo.CreateParticipant(c, p, pin)
}
func (s *Service) DeleteParticipant(c context.Context, id int64) error {
	return s.repo.DeleteParticipant(c, id)
}
func (s *Service) UpdateParticipantPIN(c context.Context, id int64, pin string) (domain.Participant, error) {
	if e := domain.ValidatePIN(pin); e != nil {
		return domain.Participant{}, e
	}
	return s.repo.UpdateParticipantPIN(c, id, pin)
}
func (s *Service) ListChores(c context.Context) ([]domain.Chore, error) { return s.repo.ListChores(c) }
func (s *Service) CreateChore(c context.Context, v domain.Chore) (domain.Chore, error) {
	return s.repo.CreateChore(c, v)
}
func (s *Service) UpdateChore(c context.Context, v domain.Chore) (domain.Chore, error) {
	return s.repo.UpdateChore(c, v)
}
func (s *Service) ListAssignments(c context.Context) ([]domain.Assignment, error) {
	return s.repo.ListAssignments(c)
}
func (s *Service) CreateAssignment(c context.Context, a, b int64) (domain.Assignment, error) {
	return s.repo.CreateAssignment(c, a, b)
}
func (s *Service) ListTasks(c context.Context, d time.Time) ([]domain.Task, error) {
	return s.repo.ListTasks(c, d)
}
func (s *Service) ListWeekPlan(c context.Context, d time.Time) ([]domain.WeekPlanItem, error) {
	return s.repo.ListWeekPlan(c, d)
}
func (s *Service) CompleteTask(c context.Context, p domain.Principal, id int64) (domain.Task, error) {
	owner, e := s.repo.TaskOwner(c, id)
	if e != nil {
		return domain.Task{}, e
	}
	if !domain.CanComplete(p, owner) {
		return domain.Task{}, domain.ErrForbidden
	}
	return s.repo.CompleteTask(c, id, p.ParticipantID)
}
func (s *Service) ConfirmTask(c context.Context, p domain.Principal, id int64, r int, comment string) (domain.Task, error) {
	if !p.IsParent() {
		return domain.Task{}, domain.ErrForbidden
	}
	return s.repo.ConfirmTask(c, id, p.ParticipantID, r, comment)
}
func (s *Service) Leaderboard(c context.Context, p string, d time.Time) ([]domain.LeaderboardEntry, error) {
	return s.repo.Leaderboard(c, p, d)
}
func (s *Service) ListBehaviorRatings(c context.Context, d time.Time) ([]domain.BehaviorRating, error) {
	return s.repo.ListBehaviorRatings(c, d)
}
func (s *Service) RateBehavior(c context.Context, p domain.Principal, d time.Time, target int64, r int, comment string) (domain.BehaviorRating, error) {
	if p.ParticipantID <= 0 || (!p.IsParent() && p.Role != domain.RoleChild) {
		return domain.BehaviorRating{}, domain.ErrForbidden
	}
	return s.repo.RateBehavior(c, d, p.ParticipantID, target, r, comment)
}
func (s *Service) ListRewards(c context.Context) ([]domain.Reward, error) {
	return s.repo.ListRewards(c)
}
func (s *Service) CreateReward(c context.Context, v domain.Reward) (domain.Reward, error) {
	return s.repo.CreateReward(c, v)
}
func (s *Service) DeleteReward(c context.Context, id int64) error { return s.repo.DeleteReward(c, id) }
func (s *Service) ExportBackup(c context.Context) (any, error)    { return s.repo.ExportBackup(c) }
func (s *Service) ImportBackup(c context.Context, payload []byte) error {
	var b BackupData
	if e := json.Unmarshal(payload, &b); e != nil {
		return e
	}
	return s.repo.ImportBackup(c, b)
}
