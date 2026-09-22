package domain

import (
	"math"
	"testing"
)

func TestSportValidationAndOwnership(t *testing.T) {
	valid := func() FamilyEntry {
		return FamilyEntry{Kind: "sport", AuthorID: 1, FamilyDraft: FamilyDraft{Title: "Бег", Date: "2026-09-22", ParticipantIDs: []int64{2}, Sport: &SportSession{Activity: "Бег", Minutes: 30, DistanceKm: 5, Exercises: []SportExercise{{Name: "Присед", Sets: 3, Reps: 10}}}}}
	}
	for _, mutate := range []func(*FamilyEntry){func(e *FamilyEntry) { e.Sport = nil }, func(e *FamilyEntry) { e.ParticipantIDs = nil }, func(e *FamilyEntry) { e.Date = "2026-02-30" }, func(e *FamilyEntry) { e.Sport.Minutes = 0 }, func(e *FamilyEntry) { e.Sport.DistanceKm = -1 }, func(e *FamilyEntry) { e.Sport.Minutes = math.NaN() }, func(e *FamilyEntry) { e.Sport.Exercises[0].Sets = 0 }, func(e *FamilyEntry) { e.Sport.Exercises[0].WeightKg = math.Inf(1) }, func(e *FamilyEntry) { e.Kind = "habit" }} {
		e := valid()
		mutate(&e)
		if e.Validate() == nil {
			t.Fatalf("invalid sport accepted: %+v", e)
		}
	}
	e := valid()
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
	if !e.CanEdit(Principal{ParticipantID: 2, Role: RoleChild}) || e.CanEdit(Principal{ParticipantID: 1, Role: RoleChild}) || !e.CanEdit(Principal{ParticipantID: 1, Role: RoleParent}) {
		t.Fatal("invalid sports ownership")
	}
}
