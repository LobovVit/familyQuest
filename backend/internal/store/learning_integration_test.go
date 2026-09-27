package store

import (
	"context"
	"errors"
	"github.com/lobov/familyquest/backend/internal/application"
	"github.com/lobov/familyquest/backend/internal/domain"
	"os"
	"sync"
	"testing"
	"time"
)

func TestLearningRewardsLifecycle(t *testing.T) {
	url := os.Getenv("FAMILYQUEST_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	s, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	t.Chdir("../..")
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `truncate participants,chores,rewards cascade`); err != nil {
		t.Fatal(err)
	}
	parent, err := s.CreateParticipant(ctx, domain.Participant{Name: "Parent", Role: domain.RoleParent}, "739281")
	if err != nil {
		t.Fatal(err)
	}
	child, err := s.CreateParticipant(ctx, domain.Participant{Name: "Child", Role: domain.RoleChild}, "391827")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateParticipant(ctx, domain.Participant{Name: "Other", Role: domain.RoleChild}, "482739")
	if err != nil {
		t.Fatal(err)
	}
	p := domain.Principal{ParticipantID: parent.ID, Role: parent.Role}
	c := domain.Principal{ParticipantID: child.ID, Role: child.Role}
	app := application.New(s, nil)
	date := "2026-09-23"
	sport := domain.FamilyDraft{Title: "Бег", Date: date, ParticipantIDs: []int64{child.ID}, Sport: &domain.SportSession{Activity: "Бег", Minutes: 30}}
	for range 2 {
		if _, err = app.CreateFamilyEntry(ctx, p, "sport", sport); err != nil {
			t.Fatal(err)
		}
	}
	habit, err := app.CreateFamilyEntry(ctx, p, "habit", domain.FamilyDraft{Title: "Прогулка", Weekdays: []int{3}, ParticipantIDs: []int64{child.ID}})
	if err != nil {
		t.Fatal(err)
	}
	habit, err = app.FamilyAction(ctx, c, habit.ID, domain.FamilyCommand{Version: habit.Version, Action: "checkin", Date: date})
	if err != nil {
		t.Fatal(err)
	}
	habit, err = app.FamilyAction(ctx, c, habit.ID, domain.FamilyCommand{Version: habit.Version, Action: "checkin", Date: date})
	if err != nil {
		t.Fatal(err)
	}
	habit, err = app.FamilyAction(ctx, p, habit.ID, domain.FamilyCommand{Version: habit.Version, Action: "archive"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = app.FamilyAction(ctx, p, habit.ID, domain.FamilyCommand{Version: habit.Version, Action: "restore"}); err != nil {
		t.Fatal(err)
	}
	adventure, err := app.CreateFamilyEntry(ctx, p, "adventure", domain.FamilyDraft{Title: "Поход", Date: date, ParticipantIDs: []int64{parent.ID, child.ID}, Steps: []domain.FamilyStep{{Title: "Идти"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = app.FamilyAction(ctx, p, adventure.ID, domain.FamilyCommand{Version: adventure.Version, Action: "step", Date: date}); err != nil {
		t.Fatal(err)
	}
	rewards, err := s.ActivityRewards(ctx, child.ID)
	if err != nil || len(rewards) != 3 {
		t.Fatal("family rewards", rewards, err)
	}
	settings := domain.MathSettings{Operation: "*", Level: "columnar", AnswerMode: "input", DivisionMode: "full"}
	view, err := app.StartMath(ctx, c, settings)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = app.StartMath(ctx, p, settings); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal("parent started child training", err)
	}
	same, err := app.StartMath(ctx, c, settings)
	if err != nil || same.ID != view.ID {
		t.Fatal("duplicate active session", err)
	}
	sessions, err := s.ListMathSessions(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	session := sessions[0]
	_, values := session.Questions[0].Work(settings)
	if _, err = s.AnswerMath(ctx, other.ID, session.ID, 0, values, time.Now()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("foreign math", err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := s.AnswerMath(ctx, child.ID, session.ID, 0, values, time.Now())
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	rewards, err = s.ActivityRewards(ctx, child.ID)
	if err != nil || len(rewards) != 4 {
		t.Fatal("duplicate math reward", rewards, err)
	}
	leaderboard, err := s.Leaderboard(ctx, "day", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range leaderboard {
		if e.ParticipantID == child.ID {
			found = e.Reward >= 3
		}
	}
	if !found {
		t.Fatal("math not in leaderboard", leaderboard)
	}
	closed, err := app.FinishMath(ctx, c, session.ID)
	if err != nil || !closed.Finished || closed.Question != nil {
		t.Fatal("close session", err)
	}
	if _, err = s.AnswerMath(ctx, child.ID, session.ID, 1, values, time.Now()); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("closed session accepted answer", err)
	}
	backup, err := app.ExportBackup(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if backup.Version != application.BackupVersion || len(backup.MathSessions) != 1 || len(backup.ActivityRewards) != 5 {
		t.Fatal("incomplete backup", len(backup.ActivityRewards))
	}
	if err = s.ImportBackup(ctx, backup); err != nil {
		t.Fatal("restore", err)
	}
	restored, err := s.ListMathSessions(ctx, child.ID)
	if err != nil || len(restored) != 1 || len(restored[0].Answers) != 1 {
		t.Fatal("math restore", err)
	}
	if _, err = s.AnswerMath(ctx, child.ID, session.ID, 0, values, time.Now()); err != nil {
		t.Fatal(err)
	}
	rewards, err = s.ActivityRewards(ctx, child.ID)
	if err != nil || len(rewards) != 4 {
		t.Fatal("restore replay duplicated rewards", err)
	}
	invalid := backup
	invalid.ActivityRewards = append([]domain.ActivityReward{}, backup.ActivityRewards...)
	invalid.ActivityRewards[0].Stars = 999
	if err = s.ImportBackup(ctx, invalid); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatal("forged reward accepted", err)
	}
	backup.Version = 2
	backup.MathSessions = nil
	backup.ActivityRewards = nil
	if err = s.ImportBackup(ctx, backup); err != nil {
		t.Fatal("legacy restore", err)
	}
	restored, err = s.ListMathSessions(ctx, child.ID)
	if err != nil || len(restored) != 0 {
		t.Fatal("legacy restore retained sessions", err)
	}
	// The budget belongs to the participant and calendar day, not to a session.
	now := time.Date(2026, 9, 24, 20, 30, 0, 0, time.UTC) // 23:30 Minsk
	for round := 0; round < 3; round++ {
		view, err := app.StartMath(ctx, c, settings)
		if err != nil {
			t.Fatal(err)
		}
		sessions, err := s.ListMathSessions(ctx, child.ID)
		if err != nil {
			t.Fatal(err)
		}
		var training domain.MathSession
		for _, v := range sessions {
			if v.ID == view.ID {
				training = v
			}
		}
		total := 0
		for i, q := range training.Questions {
			_, answer := q.Work(settings)
			saved, err := s.AnswerMath(ctx, child.ID, view.ID, i, answer, now)
			if err != nil {
				t.Fatal(err)
			}
			if !saved.Answers[i].Correct {
				t.Fatal("budget changed correctness")
			}
			total += saved.Answers[i].Stars
			if round == 0 && i == 7 && saved.Answers[i].Stars != 5 {
				t.Fatal("partial final reward")
			}
		}
		want := domain.MathDailyStarLimit
		if round == 1 {
			want = 0
		}
		if total != want {
			t.Fatalf("round %d stars %d want %d", round, total, want)
		}
		if round == 1 {
			now = now.Add(time.Hour)
		} // reset at Minsk midnight
	}
	balanced, err := app.ExportBackup(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ImportBackup(ctx, balanced); err != nil {
		t.Fatal("capped reward restore", err)
	}

}
