package domain

import (
	"reflect"
	"testing"
	"time"
)

func TestMathColumnAlgorithms(t *testing.T) {
	cases := []struct {
		op       string
		a, b     int
		expected map[string]int
	}{
		{"+", 58, 67, map[string]int{"carry_0": 1, "carry_1": 1, "answer": 125}},
		{"-", 300, 127, map[string]int{"carry_0": 1, "carry_1": 1, "carry_2": 0, "answer": 173}},
		{"*", 24, 13, map[string]int{"mulcarry_0_0": 1, "mulcarry_0_1": 0, "partial_0": 72, "mulcarry_1_0": 0, "mulcarry_1_1": 0, "partial_1": 24, "answer": 312}},
		{":", 816, 8, map[string]int{"product_0": 8, "remainder_0": 0, "bring_1": 1, "product_1": 0, "remainder_1": 1, "bring_2": 6, "product_2": 16, "remainder_2": 0, "answer": 102}},
	}
	for _, tc := range cases {
		t.Run(tc.op, func(t *testing.T) {
			fields, got := (MathQuestion{Left: tc.a, Right: tc.b}).Work(MathSettings{Operation: tc.op, Level: "columnar", DivisionMode: "full"})
			if len(fields) != len(tc.expected) || !reflect.DeepEqual(got, tc.expected) {
				t.Fatalf("got %+v want %+v", got, tc.expected)
			}
		})
	}
}
func TestMathGenerationAndIdempotence(t *testing.T) {
	for _, op := range []string{"+", "-", "*", ":"} {
		for _, level := range []string{"easy", "medium", "hard", "columnar"} {
			for _, mode := range []string{"input", "choice"} {
				if level == "columnar" && mode == "choice" {
					continue
				}
				settings := MathSettings{Operation: op, Level: level, AnswerMode: mode, DivisionMode: "full", AllowNegative: true}
				s := NewMathSession("0123456789abcdef0123456789abcdef", 2, settings, time.Now())
				if err := s.Validate(); err != nil {
					t.Fatal(settings, err)
				}
				for i, q := range s.Questions {
					_, answer := q.Work(settings)
					if err := s.Submit(i, answer, time.Now()); err != nil {
						t.Fatal(err)
					}
					if err := s.Submit(i, map[string]int{"answer": 9999}, time.Now()); err != nil {
						t.Fatal(err)
					}
				}
				if s.View().Correct != len(s.Questions) || s.View().Stars != len(s.Questions)*settings.Stars() {
					t.Fatal("incorrect rewards", s.View())
				}
				if err := s.Validate(); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}
func TestMathWorkMustBeCorrectAndOrdered(t *testing.T) {
	settings := MathSettings{Operation: "+", Level: "columnar", AnswerMode: "input", DivisionMode: "full"}
	s := NewMathSession("0123456789abcdef0123456789abcdef", 2, settings, time.Now())
	s.Questions[0] = MathQuestion{Left: 58, Right: 67, Options: []int{}}
	if s.Submit(1, map[string]int{}, time.Now()) != ErrConflict {
		t.Fatal("accepted out of order")
	}
	if s.Submit(0, map[string]int{"answer": 125}, time.Now()) != ErrInvalidInput {
		t.Fatal("accepted missing work")
	}
	if err := s.Submit(0, map[string]int{"answer": 125, "carry_0": 0, "carry_1": 1}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if s.Answers[0].Correct || s.Answers[0].Stars != 0 {
		t.Fatal("wrong carry rewarded")
	}
}

func TestBalancedMathRewards(t *testing.T) {
	cases := []struct {
		level, op, mode, division string
		want                      int
	}{
		{"easy", "+", "input", "full", 1}, {"easy", "*", "input", "full", 2},
		{"medium", "+", "choice", "full", 1}, {"medium", ":", "input", "full", 3},
		{"hard", "+", "input", "full", 3}, {"hard", "*", "choice", "full", 3},
		{"columnar", "+", "input", "full", 3}, {"columnar", "*", "input", "full", 4},
		{"columnar", ":", "input", "result", 3}, {"columnar", ":", "input", "steps", 4}, {"columnar", ":", "input", "full", 5},
	}
	for _, tc := range cases {
		settings := MathSettings{Operation: tc.op, Level: tc.level, AnswerMode: tc.mode, DivisionMode: tc.division}
		if got := settings.Stars(); got != tc.want+1 {
			t.Fatalf("%+v: %d", tc, got)
		}
	}
	settings := MathSettings{Operation: "*", Level: "columnar", AnswerMode: "input", DivisionMode: "full"}
	s := NewMathSession("0123456789abcdef0123456789abcdef", 2, settings, time.Now())
	s.RewardVersion = 0 // Old backups and already-started sessions retain their scale.
	_, values := s.Questions[0].Work(settings)
	if err := s.Submit(0, values, time.Now()); err != nil {
		t.Fatal(err)
	}
	if s.Answers[0].Stars != 3 || s.Validate() != nil {
		t.Fatal("legacy reward changed")
	}
}

func TestMathRewardBudgetPolicy(t *testing.T) {
	settings := MathSettings{Operation: "*", Level: "columnar", AnswerMode: "input", DivisionMode: "full"}
	for _, earned := range []int{38, 40, 50} {
		session := NewMathSession("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 2, settings, time.Now())
		_, answer := session.Questions[0].Work(settings)
		if err := session.Submit(0, answer, time.Now()); err != nil {
			t.Fatal(err)
		}
		reward := session.AwardAnswer(0, earned)
		if earned == 38 {
			if reward == nil || reward.Stars != 2 || session.Answers[0].Stars != 2 {
				t.Fatal("partial budget")
			}
		} else if reward != nil || session.Answers[0].Stars != 0 {
			t.Fatal("budget exceeded")
		}
		if !session.Answers[0].Correct {
			t.Fatal("cap changed correctness")
		}
	}
}
