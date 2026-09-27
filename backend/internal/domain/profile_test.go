package domain

import (
	"testing"
	"time"
)

func TestAgePolicyBoundariesAndOverrides(t *testing.T) {
	for _, tc := range []struct {
		date, birth, math, reading string
		age                        int
	}{
		{"2026-09-25", "2018-09-26", "easy", "phrases", 7},
		{"2026-09-26", "2018-09-26", "medium", "sentences", 8},
		{"2026-09-26", "2016-09-26", "hard", "advanced", 10},
		{"2025-02-28", "2016-02-29", "medium", "sentences", 8},
		{"2025-03-01", "2016-02-29", "medium", "sentences", 9},
	} {
		now, _ := time.Parse("2006-01-02", tc.date)
		p := LearningProfile{BirthDate: tc.birth}.Policy(now)
		if p.Age == nil || *p.Age != tc.age || p.MathLevel != tc.math || p.ReadingLevel != tc.reading {
			t.Fatalf("%+v: %+v", tc, p)
		}
	}
	now := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	if e := (LearningProfile{BirthDate: "2026-09-27"}).Validate(now); e == nil {
		t.Fatal("future accepted")
	}
	if e := (LearningProfile{BirthDate: "2026-02-30"}).Validate(now); e == nil {
		t.Fatal("invalid date accepted")
	}
	p := LearningProfile{BirthDate: "2020-01-01", MathLevel: "hard", ReadingLevel: "sentences"}.Policy(now)
	if !p.AllowsMath(MathSettings{Level: "medium"}) || p.AllowsMath(MathSettings{Level: "columnar"}) || p.AllowsReading("advanced") {
		t.Fatal("override boundary", p)
	}
	if (LearningProfile{}).Policy(now).Age != nil {
		t.Fatal("invented age")
	}
}
