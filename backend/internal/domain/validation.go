package domain

import (
	"strings"
	"time"
)

func ValidateRating(rating int) error {
	if rating < 1 || rating > 5 {
		return ErrInvalidRating
	}
	return nil
}

func NormalizeChore(c Chore) (Chore, error) {
	c.Title = strings.TrimSpace(c.Title)
	if c.BenefitType == "" {
		c.BenefitType = "self"
	}
	if c.ExecutionMode == "" {
		c.ExecutionMode = "assigned"
	}
	if c.Title == "" || c.BaseValue <= 0 || !oneOf(c.Schedule, "once", "daily", "weekly", "monthly") || !oneOf(c.TimeWindow, "", "morning", "day", "evening") || !oneOf(c.BenefitType, "self", "family", "care", "home") || !oneOf(c.ExecutionMode, "assigned", "together", "adult_child", "anyone") || !validIDs(c.ParticipantIDs) {
		return c, ErrInvalidInput
	}
	if err := ValidateOnceDate(c.Schedule, c.OnceDate); err != nil {
		return c, err
	}
	return c, nil
}

func ValidateOnceDate(schedule, date string) error {
	if date == "" {
		return nil
	}
	value, err := time.Parse(time.DateOnly, date)
	if schedule != "once" || err != nil || value.Year() < 2000 || value.Year() > 2100 || value.Format(time.DateOnly) != date {
		return ErrInvalidInput
	}
	return nil
}

func NormalizeReward(r Reward) (Reward, error) {
	r.Title = strings.TrimSpace(r.Title)
	if r.Period == "" {
		r.Period = "week"
	}
	if r.RewardType == "" {
		r.RewardType = "champion"
	}
	if r.RewardType != "stars" {
		r.StarCost = 0
	}
	if r.RewardType != "smiles" {
		r.SmileCost = 0
	}
	if r.Title == "" || !oneOf(r.Period, "day", "week", "month") || !oneOf(r.RewardType, "champion", "stars", "smiles") || (r.RewardType == "stars" && r.StarCost <= 0) || (r.RewardType == "smiles" && r.SmileCost <= 0) || !validIDs(r.ParticipantIDs) {
		return r, ErrInvalidInput
	}
	return r, nil
}
func oneOf(value string, values ...string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
func validIDs(ids []int64) bool {
	for _, id := range ids {
		if id <= 0 {
			return false
		}
	}
	return true
}
