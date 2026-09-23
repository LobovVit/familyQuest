package application

import (
	"context"
	"errors"
	"github.com/lobov/familyquest/backend/internal/domain"
	"testing"
	"time"
)

type testRepository struct {
	Repository
	participant domain.Participant
	err         error
}

func (r testRepository) GetParticipant(context.Context, int64) (domain.Participant, error) {
	return r.participant, r.err
}

type testTokens struct{ principal domain.Principal }

func (t testTokens) IssueConfirmation(domain.Principal) (string, error) { return "proof", nil }
func (t testTokens) Issue(domain.Participant) (string, error)           { return "token", nil }
func (t testTokens) Parse(string) (domain.Principal, error)             { return t.principal, nil }
func TestSessionRechecksParticipant(t *testing.T) {
	for _, tc := range []struct {
		name string
		p    domain.Participant
		err  error
		want error
	}{
		{"active", domain.Participant{ID: 1, Role: domain.RoleParent, Active: true}, nil, nil},
		{"deleted", domain.Participant{}, domain.ErrUnauthorized, domain.ErrUnauthorized},
		{"inactive", domain.Participant{ID: 1, Role: domain.RoleParent}, nil, domain.ErrUnauthorized},
		{"changed role", domain.Participant{ID: 1, Role: domain.RoleChild, Active: true}, nil, domain.ErrUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := New(testRepository{participant: tc.p, err: tc.err}, testTokens{domain.Principal{ParticipantID: 1, Role: domain.RoleParent}})
			_, err := s.ParseToken(context.Background(), "token")
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}
func TestMutationsRequireParentAtApplicationBoundary(t *testing.T) {
	s := New(nil, nil)
	ctx := context.Background()
	for _, actor := range []domain.Principal{{}, {ParticipantID: 2, Role: domain.RoleChild}, {Role: domain.RoleParent}} {
		calls := []func() error{
			func() error { _, e := s.CreateParticipant(ctx, actor, domain.Participant{}, "123456"); return e },
			func() error { return s.DeleteParticipant(ctx, actor, 1) },
			func() error { _, e := s.UpdateParticipantPIN(ctx, actor, 1, "123456"); return e },
			func() error { _, e := s.CreateChore(ctx, actor, domain.Chore{}); return e },
			func() error { _, e := s.UpdateChore(ctx, actor, domain.Chore{}); return e },
			func() error { _, e := s.CreateAssignment(ctx, actor, 1, 1); return e },
			func() error { _, e := s.CreateReward(ctx, actor, domain.Reward{}); return e },
			func() error { return s.DeleteReward(ctx, actor, 1) },
			func() error { _, e := s.ExportBackup(ctx, actor); return e },
			func() error { return s.ImportBackup(ctx, actor, BackupData{}) },
			func() error { _, e := s.ConfirmTask(ctx, actor, 1, 5, ""); return e },
		}
		for i, call := range calls {
			if e := call(); !errors.Is(e, domain.ErrForbidden) {
				t.Fatalf("call %d actor %+v: %v", i, actor, e)
			}
		}
	}
}
func TestInvalidRatingStopsBeforeRepository(t *testing.T) {
	s := New(nil, nil)
	_, err := s.RateBehavior(context.Background(), domain.Principal{ParticipantID: 2, Role: domain.RoleChild}, time.Now(), 3, 6, "")
	if !errors.Is(err, domain.ErrInvalidRating) {
		t.Fatal(err)
	}
}
