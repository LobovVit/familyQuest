package domain

import (
	"slices"
	"time"
)

type LearningProfile struct {
	BirthDate    string `json:"birthDate"`
	MathLevel    string `json:"mathLevel"`
	ReadingLevel string `json:"readingLevel"`
}
type LearningPolicy struct {
	Age          *int   `json:"age,omitempty"`
	MathLevel    string `json:"mathLevel"`
	ReadingLevel string `json:"readingLevel"`
	Version      int    `json:"version"`
}

func (p LearningProfile) Validate(now time.Time) error {
	if p.BirthDate != "" {
		d, e := time.Parse("2006-01-02", p.BirthDate)
		if e != nil || d.Format("2006-01-02") > now.In(time.FixedZone("Europe/Minsk", 10800)).Format("2006-01-02") || d.Year() < 1900 {
			return ErrInvalidInput
		}
	}
	if !slices.Contains([]string{"", "easy", "medium", "hard", "columnar"}, p.MathLevel) || !slices.Contains([]string{"", "phrases", "sentences", "advanced"}, p.ReadingLevel) {
		return ErrInvalidInput
	}
	return nil
}
func (p LearningProfile) Policy(now time.Time) LearningPolicy {
	v := LearningPolicy{MathLevel: "easy", ReadingLevel: "phrases", Version: 1}
	if d, e := time.Parse("2006-01-02", p.BirthDate); e == nil {
		n := now.In(time.FixedZone("Europe/Minsk", 10800))
		age := n.Year() - d.Year()
		if n.Month() < d.Month() || (n.Month() == d.Month() && n.Day() < d.Day()) {
			age--
		}
		v.Age = &age
		if age >= 10 {
			v.MathLevel = "hard"
			v.ReadingLevel = "advanced"
		} else if age >= 8 {
			v.MathLevel = "medium"
			v.ReadingLevel = "sentences"
		}
	}
	if p.MathLevel != "" {
		v.MathLevel = p.MathLevel
	}
	if p.ReadingLevel != "" {
		v.ReadingLevel = p.ReadingLevel
	}
	return v
}
func (p LearningPolicy) AllowsMath(s MathSettings) bool {
	if s.Level == "columnar" {
		return p.MathLevel == "columnar"
	}
	levels := map[string]int{"easy": 1, "medium": 2, "hard": 3, "columnar": 3}
	return levels[s.Level] > 0 && levels[s.Level] <= levels[p.MathLevel]
}
func (p LearningPolicy) AllowsReading(level string) bool {
	levels := map[string]int{"phrases": 1, "sentences": 2, "advanced": 3}
	return levels[level] > 0 && levels[level] <= levels[p.ReadingLevel]
}
