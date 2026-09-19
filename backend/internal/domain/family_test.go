package domain

import (
	"errors"
	"testing"
	"time"
)

func TestFamilyProgressRules(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	child := Principal{ParticipantID: 2, Role: RoleChild}
	parent := Principal{ParticipantID: 1, Role: RoleParent}
	base := FamilyEntry{Version: 1, Kind: "proposal", AuthorID: 2, FamilyDraft: FamilyDraft{Title: "Наше кафе", Date: "2026-09-19", Steps: []FamilyStep{{Title: "Меню", ParticipantID: 2}, {Title: "Ужин", ParticipantID: 1}}}, Events: []FamilyEvent{}}
	command := FamilyCommand{Version: 1, Action: "step", Date: "2026-09-19", Step: 0}
	if err := base.ApplyFamilyCommand(child, command, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("unapproved proposal: %v", err)
	}
	if err := base.ApplyFamilyCommand(child, FamilyCommand{Version: 1, Action: "approve"}, now); !errors.Is(err, ErrForbidden) {
		t.Fatalf("child approval: %v", err)
	}
	if err := base.ApplyFamilyCommand(parent, FamilyCommand{Version: 1, Action: "approve"}, now); err != nil {
		t.Fatal(err)
	}
	command.Step = 1
	if err := base.ApplyFamilyCommand(child, command, now); !errors.Is(err, ErrForbidden) {
		t.Fatalf("other participant step: %v", err)
	}
	command.Step = 0
	if err := base.ApplyFamilyCommand(child, command, now); err != nil {
		t.Fatal(err)
	}
	if err := base.ApplyFamilyCommand(child, command, now); err != nil {
		t.Fatal(err)
	}
	if len(base.Events) != 1 || base.Events[0].ActorID != child.ParticipantID {
		t.Fatalf("duplicate or incorrect actor: %+v", base.Events)
	}
	command.Version = 0
	if err := base.ApplyFamilyCommand(child, command, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale version: %v", err)
	}
	if err := base.ApplyFamilyCommand(Principal{ParticipantID: 3, Role: RoleSchool}, command, now); !errors.Is(err, ErrForbidden) {
		t.Fatalf("school access: %v", err)
	}
}
func TestFamilyHabitKeepsHistoryAfterGapAndArchive(t *testing.T) {
	e := FamilyEntry{Version: 1, Kind: "habit", AuthorID: 2, FamilyDraft: FamilyDraft{Title: "Чтение", Weekdays: []int{1, 3, 5}}}
	p := Principal{ParticipantID: 2, Role: RoleChild}
	now := time.Now()
	for _, date := range []string{"2026-01-01", "2026-01-01", "2026-02-01"} {
		if err := e.ApplyFamilyCommand(p, FamilyCommand{Version: 1, Action: "checkin", Date: date, Easy: true}, now); err != nil {
			t.Fatal(err)
		}
	}
	if len(e.Events) != 2 {
		t.Fatalf("history: %+v", e.Events)
	}
	if err := e.ApplyFamilyCommand(p, FamilyCommand{Version: 1, Action: "archive"}, now); err != nil {
		t.Fatal(err)
	}
	if err := e.ApplyFamilyCommand(p, FamilyCommand{Version: 1, Action: "checkin", Date: "2026-02-02"}, now); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if err := e.ApplyFamilyCommand(p, FamilyCommand{Version: 1, Action: "restore"}, now); err != nil {
		t.Fatal(err)
	}
	if len(e.Events) != 2 {
		t.Fatal("archive lost history")
	}
}
func TestFamilySkillStagesArePersonalAndSequential(t *testing.T) {
	e := FamilyEntry{Version: 1, Kind: "skill"}
	now := time.Now()
	p := Principal{ParticipantID: 2, Role: RoleChild}
	if err := e.ApplyFamilyCommand(p, FamilyCommand{Version: 1, Action: "stage", Step: 4, Date: "2026-01-01"}, now); !errors.Is(err, ErrConflict) {
		t.Fatal("skipped skill stages")
	}
	for i := 1; i <= 4; i++ {
		if err := e.ApplyFamilyCommand(p, FamilyCommand{Version: 1, Action: "stage", Step: i, Date: "2026-01-01"}, now); err != nil {
			t.Fatal(err)
		}
	}
	p.ParticipantID = 3
	if err := e.ApplyFamilyCommand(p, FamilyCommand{Version: 1, Action: "stage", Step: 1, Date: "2026-01-01"}, now); err != nil {
		t.Fatal("stage leaked between participants", err)
	}
}
func TestFamilyValidation(t *testing.T) {
	for _, e := range []FamilyEntry{
		{Kind: "habit", FamilyDraft: FamilyDraft{Title: " ", Weekdays: []int{1}}},
		{Kind: "habit", FamilyDraft: FamilyDraft{Title: "Read", Weekdays: []int{1, 1}}},
		{Kind: "memory", FamilyDraft: FamilyDraft{Title: "A", Date: "2026-02-30"}},
		{Kind: "memory", FamilyDraft: FamilyDraft{Title: "A", Date: "2026-02-01", Photo: "data:image/svg+xml;base64,PHN2Zz4="}},
		{Kind: "adventure", FamilyDraft: FamilyDraft{Title: "A", Date: "2026-02-01", ParticipantIDs: []int64{1}, Steps: []FamilyStep{{Title: "B", ParticipantID: 2}}}},
	} {
		if err := e.Validate(); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("accepted invalid: %+v", e)
		}
	}
	// Media and reflections are optional; there is no proof requirement.
	if err := (FamilyEntry{Kind: "memory", FamilyDraft: FamilyDraft{Title: "Приятный день", Date: "2026-02-01"}}).Validate(); err != nil {
		t.Fatal(err)
	}
}
