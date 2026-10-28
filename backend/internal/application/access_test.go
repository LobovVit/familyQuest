package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lobov/familyquest/backend/internal/domain"
)

func TestDelegatedMutationsAreGuardedWithoutHTTP(t *testing.T) {
	ctx := context.Background()
	// Nil ports ensure rejected calls do not reach persistence or token issuance.
	s := New(nil, nil)
	for _, role := range []string{domain.RoleParent, domain.RoleChild, domain.RoleSchool} {
		for _, readOnly := range []bool{false, true} {
			actor := domain.Principal{ParticipantID: 2, Role: role, MCPGrantID: "delegation", MCPReadOnly: readOnly}
			calls := map[string]func() error{
				"participant creation": func() error { _, e := s.CreateParticipant(ctx, actor, domain.Participant{}, "123456"); return e },
				"participant removal":  func() error { return s.DeleteParticipant(ctx, actor, 3) },
				"PIN":                  func() error { _, e := s.UpdateParticipantPIN(ctx, actor, 3, "123456", "proof"); return e },
				"parent proof":         func() error { _, e := s.ConfirmParent(ctx, actor, "123456"); return e },
				"device removal":       func() error { return s.RemoveDevice(ctx, actor, "device", "proof") },
				"chore creation":       func() error { _, e := s.CreateChore(ctx, actor, domain.Chore{}); return e },
				"chore update":         func() error { _, e := s.UpdateChore(ctx, actor, domain.Chore{}); return e },
				"assignment":           func() error { _, e := s.CreateAssignment(ctx, actor, 1, 2); return e },
				"reward creation":      func() error { _, e := s.CreateReward(ctx, actor, domain.Reward{}); return e },
				"reward removal":       func() error { return s.DeleteReward(ctx, actor, 1) },
				"behavior":             func() error { _, e := s.RateBehavior(ctx, actor, time.Now(), 3, 5, ""); return e },
				"restore":              func() error { return s.ImportBackup(ctx, actor, BackupData{}, "proof") },
				"profile":              func() error { return s.SaveProfile(ctx, actor, 2, domain.LearningProfile{}) },
				"math start":           func() error { _, e := s.StartMath(ctx, actor, domain.MathSettings{}); return e },
				"math answer":          func() error { _, e := s.AnswerMath(ctx, actor, "lesson", 0, nil); return e },
				"math finish":          func() error { _, e := s.FinishMath(ctx, actor, "lesson"); return e },
				"reading start":        func() error { return s.StartReading(ctx, actor, domain.ReadingCompletion{}) },
				"reading finish":       func() error { _, e := s.CompleteReading(ctx, actor, domain.ReadingCompletion{}); return e },
				"family creation":      func() error { _, e := s.CreateFamilyEntry(ctx, actor, "habit", domain.FamilyDraft{}); return e },
				"family edit":          func() error { _, e := s.EditFamilyEntry(ctx, actor, 1, 1, domain.FamilyDraft{}); return e },
				"family approval":      func() error { _, e := s.FamilyAction(ctx, actor, 1, domain.FamilyCommand{Action: "approve"}); return e },
				"grant creation":       func() error { _, _, e := s.CreateMCPGrant(ctx, actor, 2, "agent", false, "proof"); return e },
				"grant revocation":     func() error { return s.RemoveMCPGrant(ctx, actor, "other") },
			}
			if readOnly || role == domain.RoleSchool {
				calls["task completion"] = func() error { _, e := s.CompleteTask(ctx, actor, 1); return e }
				calls["task confirmation"] = func() error { _, e := s.ConfirmTask(ctx, actor, 1, 5, ""); return e }
				calls["family progress"] = func() error { _, e := s.FamilyAction(ctx, actor, 1, domain.FamilyCommand{Action: "checkin"}); return e }
			}
			for name, call := range calls {
				t.Run(role+"/"+map[bool]string{true: "read", false: "write"}[readOnly]+"/"+name, func(t *testing.T) {
					if err := call(); !errors.Is(err, domain.ErrForbidden) {
						t.Fatalf("delegated access escaped the use case boundary: %v", err)
					}
				})
			}
		}
	}
}

type progressRepository struct {
	Repository
	entry     domain.FamilyEntry
	writes    int
	accessErr error
}

func (r *progressRepository) CheckWriteAccess(context.Context) error { return r.accessErr }
func (r *progressRepository) GetParticipant(_ context.Context, id int64) (domain.Participant, error) {
	return domain.Participant{ID: id, Role: domain.RoleChild, Active: true}, nil
}
func (r *progressRepository) GetFamilyEntry(context.Context, int64) (domain.FamilyEntry, error) {
	return r.entry, nil
}
func (r *progressRepository) SaveFamilyEntry(_ context.Context, e domain.FamilyEntry) (domain.FamilyEntry, error) {
	r.writes++
	r.entry = e
	return e, nil
}

func TestDelegatedProgressPreservesFamilySubscriptionAndClock(t *testing.T) {
	now := time.Date(2026, 10, 28, 21, 0, 0, 0, time.UTC)
	actor := domain.Principal{FamilyID: 10, ParticipantID: 2, Role: domain.RoleChild, MCPGrantID: "agent"}
	for _, name := range []string{"allowed", "other family", "expired", "future date"} {
		t.Run(name, func(t *testing.T) {
			repo := &progressRepository{entry: domain.FamilyEntry{ID: 1, Version: 1, Kind: "habit"}}
			s := NewForFamily(repo, nil, 10)
			s.now = func() time.Time { return now }
			p := actor
			date := "2026-10-29"
			if name == "other family" {
				p.FamilyID++
			}
			if name == "expired" {
				repo.accessErr = domain.ErrForbidden
			}
			if name == "future date" {
				date = "2026-10-30"
			}
			v, err := s.FamilyAction(context.Background(), p, 1, domain.FamilyCommand{Version: 1, Action: "checkin", Date: date})
			if name == "allowed" {
				if err != nil || repo.writes != 1 || len(v.Events) != 1 || !v.Events[0].CreatedAt.Equal(now) {
					t.Fatalf("valid delegated progress: %+v, %v", v, err)
				}
			} else if err == nil || repo.writes != 0 {
				t.Fatalf("invalid progress saved: %v", err)
			}
		})
	}
}

func TestParseTokenRejectsOtherFamilyBeforeRepository(t *testing.T) {
	for _, family := range []int64{0, 10} {
		s := NewForFamily(nil, testTokens{domain.Principal{FamilyID: family + 1, ParticipantID: 1, Role: domain.RoleParent}}, family)
		if _, err := s.ParseToken(context.Background(), "token"); !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("service %d accepted a foreign token: %v", family, err)
		}
	}
}

func TestSportUsesInjectedMinskDate(t *testing.T) {
	for _, tc := range []struct {
		date    string
		allowed bool
	}{
		{"2040-10-29", true}, {"2040-10-30", false},
	} {
		t.Run(tc.date, func(t *testing.T) {
			repo := &progressRepository{}
			s := New(repo, nil)
			s.now = func() time.Time { return time.Date(2040, 10, 28, 21, 0, 0, 0, time.UTC) }
			_, err := s.CreateFamilyEntry(context.Background(), domain.Principal{ParticipantID: 2, Role: domain.RoleChild}, "sport", domain.FamilyDraft{
				Title: "Бег", Date: tc.date, ParticipantIDs: []int64{2}, Sport: &domain.SportSession{Activity: "Бег", Minutes: 30},
			})
			if tc.allowed {
				if err != nil || repo.writes != 1 {
					t.Fatalf("today rejected: %v", err)
				}
			} else if !errors.Is(err, domain.ErrInvalidInput) || repo.writes != 0 {
				t.Fatalf("future sport saved: %v", err)
			}
		})
	}
}
