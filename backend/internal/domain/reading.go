package domain

import (
	"encoding/hex"
	"fmt"
	"time"
)

const ReadingDailyStarLimit = 30

type ReadingCompletion struct {
	ID    string `json:"id"`
	Level string `json:"level"`
}

func (c ReadingCompletion) Validate() error {
	raw, err := hex.DecodeString(c.ID)
	if err != nil || len(raw) != 16 || hex.EncodeToString(raw) != c.ID || c.Stars() == 0 {
		return ErrInvalidInput
	}
	return nil
}
func (c ReadingCompletion) Stars() int {
	switch c.Level {
	case "phrases":
		return 6
	case "sentences":
		return 9
	case "advanced":
		return 12
	}
	return 0
}
func (c ReadingCompletion) Title() string {
	return fmt.Sprintf("Чтение · уровень %d", c.Stars()/3-1)
}

// Reading records self-reported practice, not verified pronunciation.
// Чтение отмечает самостоятельное занятие, а не проверку произношения.
func (c ReadingCompletion) Reward(owner int64, earned int, now time.Time) ActivityReward {
	return ActivityReward{Source: "reading", SourceKey: c.ID, ParticipantID: owner, Date: now.In(time.FixedZone("Europe/Minsk", 3*60*60)).Format("2006-01-02"), Stars: min(c.Stars(), max(0, ReadingDailyStarLimit-earned)), Title: c.Title()}
}
