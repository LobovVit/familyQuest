package domain

import "strings"

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
	return c, nil
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
