package store

import (
	"context"
	"errors"
	"github.com/lobov/familyquest/backend/internal/application"
	"github.com/lobov/familyquest/backend/internal/domain"
	"os"
	"sync"
	"testing"
)

func TestFamilyPostgresLifecycle(t *testing.T) {
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
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `truncate participants,chores,rewards cascade`); err != nil {
		t.Fatal(err)
	}
	parent, err := s.CreateParticipant(ctx, domain.Participant{Name: "Family parent", Role: domain.RoleParent}, "739281")
	if err != nil {
		t.Fatal(err)
	}
	child, err := s.CreateParticipant(ctx, domain.Participant{Name: "Family child", Role: domain.RoleChild}, "391827")
	if err != nil {
		t.Fatal(err)
	}
	p := domain.Principal{ParticipantID: parent.ID, Role: parent.Role}
	c := domain.Principal{ParticipantID: child.ID, Role: child.Role}
	app := application.New(s, nil)
	if _, err := app.CreateFamilyEntry(ctx, p, "habit", domain.FamilyDraft{Title: "Missing participant", Weekdays: []int{1}, ParticipantIDs: []int64{999999}}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("missing target should be invalid input, got %v", err)
	}
	e, err := app.CreateFamilyEntry(ctx, c, "adventure", domain.FamilyDraft{Title: "Наше кафе", Date: "2026-09-19", Steps: []domain.FamilyStep{{Title: "Сделать меню", ParticipantID: child.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	if e.Kind != "proposal" || e.AuthorID != child.ID || e.Version != 1 {
		t.Fatalf("create: %+v", e)
	}
	if _, err := app.FamilyAction(ctx, c, e.ID, domain.FamilyCommand{Version: e.Version, Action: "step", Date: "2026-09-19"}); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("unapproved progress", err)
	}
	e, err = app.FamilyAction(ctx, p, e.ID, domain.FamilyCommand{Version: e.Version, Action: "approve"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.EditFamilyEntry(ctx, domain.Principal{ParticipantID: 999, Role: domain.RoleChild}, e.ID, e.Version, e.FamilyDraft); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal("non-author edit", err)
	}
	// Simultaneous devices: exactly one wins; the other receives a conflict.
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := app.FamilyAction(ctx, c, e.ID, domain.FamilyCommand{Version: e.Version, Action: "step", Date: "2026-09-19"})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, domain.ErrConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("concurrency: success=%d conflict=%d", success, conflict)
	}
	saved, err := s.GetFamilyEntry(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	draft := saved.FamilyDraft
	draft.Steps = []domain.FamilyStep{{Title: "Подменить шаг"}}
	if _, err := app.EditFamilyEntry(ctx, c, e.ID, saved.Version, draft); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("changed executed step", err)
	}
	const photo = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jWZkAAAAASUVORK5CYII="
	for _, kind := range []string{"habit", "value", "skill", "council", "memory"} {
		draft := domain.FamilyDraft{Title: "Family " + kind, Description: "Вместе", Date: "2026-09-19", Weekdays: []int{1, 3}, Value: "Забота"}
		if kind == "memory" {
			draft.Photo = photo
		}
		_, err := app.CreateFamilyEntry(ctx, p, kind, draft)
		if err != nil {
			t.Fatal(kind, err)
		}
	}

	sportDraft := domain.FamilyDraft{Title: "Тренировка", Date: "2026-09-19", ParticipantIDs: []int64{child.ID}, Sport: &domain.SportSession{Activity: "Зал", Minutes: 45, Exercises: []domain.SportExercise{{Name: "Присед", Sets: 3, Reps: 10, WeightKg: 5}}}}
	sport, err := app.CreateFamilyEntry(ctx, p, "sport", sportDraft)
	if err != nil {
		t.Fatal(err)
	}
	foreign := sportDraft
	foreign.ParticipantIDs = []int64{parent.ID}
	if _, err := app.CreateFamilyEntry(ctx, c, "sport", foreign); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal("child created foreign sport", err)
	}
	if _, err := app.EditFamilyEntry(ctx, c, sport.ID, sport.Version, foreign); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal("child reassigned sport", err)
	}
	sportDraft.Description = "Самостоятельная запись"
	updatedSport, err := app.EditFamilyEntry(ctx, c, sport.ID, sport.Version, sportDraft)
	if err != nil {
		t.Fatal("child cannot edit own parent-created sport", err)
	}
	if _, err := app.EditFamilyEntry(ctx, p, sport.ID, sport.Version, sportDraft); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("stale sport accepted", err)
	}
	if _, err := app.FamilyAction(ctx, c, sport.ID, domain.FamilyCommand{Version: updatedSport.Version, Action: "archive"}); err != nil {
		t.Fatal(err)
	}
	backup, err := app.ExportBackup(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if backup.Version != application.BackupVersion || len(backup.FamilyEntries) != 7 {
		t.Fatalf("backup missing family: %d", len(backup.FamilyEntries))
	}
	if err := s.ImportBackup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	restored, err := s.GetFamilyEntry(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.Events) != 1 || restored.Events[0].ActorID != child.ID || !restored.Approved {
		t.Fatalf("restore lost progress: %+v", restored)
	}
	all, err := s.ListFamilyEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	restoredSport, err := s.GetFamilyEntry(ctx, sport.ID)
	if err != nil || restoredSport.Sport == nil || restoredSport.Sport.Exercises[0].WeightKg != 5 || !restoredSport.Archived {
		t.Fatal("backup lost sport", err)
	}
	foundPhoto := false
	for _, item := range all {
		if item.Kind == "memory" && item.Photo == photo {
			foundPhoto = true
		}
	}
	if !foundPhoto {
		t.Fatal("backup lost photo")
	}
	invalid := backup
	invalid.FamilyEntries = append([]domain.FamilyEntry{}, backup.FamilyEntries...)
	invalid.FamilyEntries[0].AuthorID = 999999
	if err := s.ImportBackup(ctx, invalid); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatal("invalid backup accepted", err)
	}
	if _, err := s.GetFamilyEntry(ctx, e.ID); err != nil {
		t.Fatal("invalid import damaged data", err)
	}
	// Old backups remain usable and explicitly replace the entire family state.
	backup.Version = 1
	backup.FamilyEntries = nil
	backup.MathSessions = nil
	backup.ActivityRewards = nil
	if err := s.ImportBackup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	entries, err := s.ListFamilyEntries(ctx)
	if err != nil || len(entries) != 0 {
		t.Fatal("v1 import", entries, err)
	}
}
