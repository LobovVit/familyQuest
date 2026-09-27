package domain

import (
	"testing"
	"time"
)

func TestReadingRewardPolicy(t *testing.T) {
	for level, want := range map[string]int{"phrases": 6, "sentences": 9, "advanced": 12} {
		c := ReadingCompletion{ID: "0123456789abcdef0123456789abcdef", Level: level}
		if c.Validate() != nil || c.Stars() != want {
			t.Fatal(c)
		}
		now := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
		r := c.Reward(1, 28, now)
		if r.Stars != 2 || r.Date != "2026-09-27" || r.Smiles != 0 {
			t.Fatal(r)
		}
		if c.Reward(1, 30, now).Stars != 0 {
			t.Fatal("cap")
		}
	}
	for _, c := range []ReadingCompletion{{ID: "bad", Level: "phrases"}, {ID: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", Level: "phrases"}, {ID: "0123456789abcdef0123456789abcdef", Level: "unknown"}} {
		if c.Validate() == nil {
			t.Fatal("invalid completion", c)
		}
	}
}
func TestMathPreviousRewardVersion(t *testing.T) {
	settings := MathSettings{Operation: "*", Level: "columnar", AnswerMode: "input", DivisionMode: "full"}
	session := NewMathSession("0123456789abcdef0123456789abcdef", 1, settings, time.Now())
	session.RewardVersion = 2
	_, answer := session.Questions[0].Work(settings)
	if err := session.Submit(0, answer, time.Now()); err != nil {
		t.Fatal(err)
	}
	if session.Answers[0].Stars != 4 {
		t.Fatal("old scale changed")
	}
	if r := session.AwardAnswer(0, 29); r == nil || r.Stars != 1 {
		t.Fatal("old cap changed", r)
	}
	if session.Validate() != nil {
		t.Fatal("old validation")
	}
}
