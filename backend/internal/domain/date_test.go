package domain

import (
	"errors"
	"testing"
	"time"
)

func TestProgressCannotBeRecordedBeforeItsMinskDate(t *testing.T) {
	actor := Principal{ParticipantID: 2, Role: RoleChild}
	for _, tc := range []struct {
		instant, date string
		allowed       bool
	}{
		{"2026-10-28T12:00:00Z", "2026-10-28", true},
		{"2026-10-28T12:00:00Z", "2026-10-29", false},
		{"2026-10-28T20:59:59Z", "2026-10-29", false},
		{"2026-10-28T21:00:00Z", "2026-10-29", true},
		{"2026-10-28T21:00:00Z", "2026-10-30", false},
	} {
		t.Run(tc.instant+"/"+tc.date, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, tc.instant)
			if err != nil {
				t.Fatal(err)
			}
			entry := FamilyEntry{Kind: "habit", Version: 1}
			err = entry.ApplyFamilyCommand(actor, FamilyCommand{Version: 1, Action: "checkin", Date: tc.date}, now)
			if tc.allowed {
				if err != nil || len(entry.Events) != 1 || !entry.Events[0].CreatedAt.Equal(now) {
					t.Fatalf("today's progress rejected: %+v, %v", entry.Events, err)
				}
			} else if !errors.Is(err, ErrInvalidInput) || len(entry.Events) != 0 {
				t.Fatalf("future progress accepted: %+v, %v", entry.Events, err)
			}
			if reward := (ReadingCompletion{Level: "phrases"}).Reward(actor.ParticipantID, 0, now); reward.Date != FamilyDate(now) {
				t.Fatal("progress and rewards use different calendars")
			}
		})
	}
}
