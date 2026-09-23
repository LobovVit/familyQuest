package application

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/lobov/familyquest/backend/internal/domain"
	"strings"
	"testing"
)

type overviewRepository struct {
	Repository
	entries []domain.FamilyEntry
}

func (r overviewRepository) ListFamilyEntries(context.Context) ([]domain.FamilyEntry, error) {
	return r.entries, nil
}
func TestFamilyOverviewProjection(t *testing.T) {
	entries := []domain.FamilyEntry{
		{ID: 1, Kind: "sport", FamilyDraft: domain.FamilyDraft{Title: "Бег", Date: "2026-09-21", Description: "PRIVATE_NOTE", Photo: "PRIVATE_PHOTO", ParticipantIDs: []int64{2}, Sport: &domain.SportSession{Activity: "Бег", Minutes: 30, DistanceKm: 5}}},
		{ID: 2, Kind: "habit", FamilyDraft: domain.FamilyDraft{Title: "Чтение", Date: "2026-09-01", Weekdays: []int{3}}, Events: []domain.FamilyEvent{{Kind: "checkin", Date: "2026-09-23", ActorID: 2, Note: "PRIVATE_EVENT"}, {Kind: "checkin", Date: "2026-09-23", ActorID: 2}, {Kind: "checkin", Date: "2026-09-22", ActorID: 3}}},
		{ID: 3, Kind: "adventure", FamilyDraft: domain.FamilyDraft{Title: "Поход", Date: "2026-09-24", Steps: []domain.FamilyStep{{Title: "PRIVATE_STEP"}, {Title: "Шаг"}}}, Events: []domain.FamilyEvent{{Kind: "step", Step: 0, Date: "2026-09-23"}, {Kind: "step", Step: 0, Date: "2026-09-23"}, {Kind: "step", Step: 1, Date: "2026-09-24"}}},
		{ID: 4, Kind: "proposal", FamilyDraft: domain.FamilyDraft{Title: "PRIVATE_PROPOSAL"}},
		{ID: 5, Kind: "memory", FamilyDraft: domain.FamilyDraft{Title: "PRIVATE_MEMORY"}},
	}
	archived := entries[0]
	archived.Archived = true
	archived.ID = 6
	old := entries[0]
	old.Date = "2026-09-20"
	old.ID = 7
	future := entries[0]
	future.Date = "2026-09-24"
	future.ID = 8
	entries = append(entries, archived, old, future)
	s := New(overviewRepository{entries: entries}, nil)
	got, err := s.FamilyOverview(context.Background(), "2026-09-23")
	if err != nil {
		t.Fatal(err)
	}
	if got.WeekStart != "2026-09-21" || len(got.Sports) != 1 || len(got.Habits) != 1 || len(got.Adventures) != 1 {
		t.Fatalf("unexpected overview: %+v", got)
	}
	if len(got.Habits[0].CompletedIDs) != 1 || got.Adventures[0].CompletedSteps != 1 {
		t.Fatal("incorrect progress", got)
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "PRIVATE_") {
		t.Fatal("private fields exposed", string(raw))
	}
	for _, date := range []string{"", "2026-02-30", "bad"} {
		if _, err := s.FamilyOverview(context.Background(), date); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatal("invalid date accepted", date, err)
		}
	}
	sunday, err := s.FamilyOverview(context.Background(), "2026-09-27")
	if err != nil || sunday.WeekStart != "2026-09-21" {
		t.Fatal("wrong Sunday week", sunday, err)
	}
}
