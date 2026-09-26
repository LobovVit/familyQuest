package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/lobov/familyquest/backend/internal/application"
	"github.com/lobov/familyquest/backend/internal/domain"
	"os"
	"sync"
	"testing"
	"time"
)

func TestReadingRewardsLifecycle(t *testing.T) {
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
	child, err := s.CreateParticipant(ctx, domain.Participant{Name: "Reader", Role: domain.RoleChild}, "391827")
	if err != nil {
		t.Fatal(err)
	}
	parent, err := s.CreateParticipant(ctx, domain.Participant{Name: "Parent", Role: domain.RoleParent}, "739281")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateParticipant(ctx, domain.Participant{Name: "Other", Role: domain.RoleChild}, "482739")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 26, 20, 59, 0, 0, time.UTC)
	completion := domain.ReadingCompletion{ID: fmt.Sprintf("%032x", 1), Level: "advanced"}
	if _, err = s.CompleteReading(ctx, parent.ID, completion, now); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal("parent", err)
	}
	var wg sync.WaitGroup
	results := make(chan domain.ActivityReward, 8)
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := s.CompleteReading(ctx, child.ID, completion, now)
			results <- r
			errs <- e
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	for r := range results {
		if r.Stars != 12 {
			t.Fatal("replay", r)
		}
	}
	rewards, err := s.ActivityRewards(ctx, child.ID)
	if err != nil || len(rewards) != 1 {
		t.Fatal("duplicate", rewards, err)
	}
	changed := completion
	changed.Level = "phrases"
	if _, err = s.CompleteReading(ctx, child.ID, changed, now); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("changed retry", err)
	}
	// Distinct concurrent completions must still share one daily budget.
	// Разные одновременные занятия используют один дневной бюджет.
	errs = make(chan error, 8)
	for i := 2; i <= 9; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, e := s.CompleteReading(ctx, child.ID, domain.ReadingCompletion{ID: fmt.Sprintf("%032x", i), Level: "advanced"}, now)
			errs <- e
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	rewards, err = s.ActivityRewards(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	total, partial := 0, false
	var zero domain.ActivityReward
	for _, r := range rewards {
		total += r.Stars
		if r.Stars == 6 {
			partial = true
		}
		if r.Stars == 0 {
			zero = r
		}
	}
	if total != 30 || !partial || zero.SourceKey == "" || len(rewards) != 9 {
		t.Fatal("budget", rewards)
	}
	tomorrow := now.Add(2 * time.Minute)
	for _, r := range []domain.ActivityReward{rewards[0], zero} {
		replay, e := s.CompleteReading(ctx, child.ID, domain.ReadingCompletion{ID: r.SourceKey, Level: "advanced"}, tomorrow)
		if e != nil || replay != r {
			t.Fatal("midnight retry", replay, e)
		}
	}
	next, e := s.CompleteReading(ctx, child.ID, domain.ReadingCompletion{ID: fmt.Sprintf("%032x", 10), Level: "sentences"}, tomorrow)
	if e != nil || next.Stars != 9 || next.Date != "2026-09-27" {
		t.Fatal("new day", next, e)
	}
	own, e := s.CompleteReading(ctx, other.ID, completion, now)
	if e != nil || own.Stars != 12 || own.ParticipantID != other.ID {
		t.Fatal("owner isolation", own, e)
	}
	board, e := s.Leaderboard(ctx, "day", now)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, row := range board {
		if row.ParticipantID == child.ID {
			found = row.Reward == 30
		}
	}
	if !found {
		t.Fatal("reading missing from leaderboard", board)
	}
	backup, e := s.ExportBackup(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if backup.Version != application.BackupVersion {
		t.Fatal("version", backup.Version)
	}
	if e = s.ImportBackup(ctx, backup); e != nil {
		t.Fatal("restore", e)
	}
	replay, e := s.CompleteReading(ctx, child.ID, domain.ReadingCompletion{ID: zero.SourceKey, Level: "advanced"}, tomorrow)
	if e != nil || replay != zero {
		t.Fatal("restore replay", replay, e)
	}
	if _, e = s.pool.Exec(ctx, `update participants set active=false where id=$1`, child.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.CompleteReading(ctx, child.ID, completion, now); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("inactive replay", e)
	}
}
