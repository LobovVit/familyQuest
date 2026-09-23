package domain

import (
	"fmt"
	"slices"
	"time"
)

type ActivityReward struct {
	Source        string `json:"source"`
	SourceKey     string `json:"sourceKey"`
	ParticipantID int64  `json:"participantId"`
	Date          string `json:"date"`
	Stars         int    `json:"stars"`
	Smiles        int    `json:"smiles"`
	Title         string `json:"title"`
}

// Reward candidates are derived from new progress only. Archive/restore and edits
// cannot award the same achievement again; persistence enforces unique sources.
func FamilyRewardCandidates(before, after FamilyEntry) []ActivityReward {
	out := []ActivityReward{}
	add := func(source, key string, id int64, date string, stars, smiles int) {
		out = append(out, ActivityReward{source, key, id, date, stars, smiles, after.Title})
	}
	if after.Archived {
		return out
	}
	if after.Kind == "sport" && before.ID == 0 && len(after.ParticipantIDs) == 1 {
		add("sport", after.Date, after.ParticipantIDs[0], after.Date, 10, 2)
	}
	if after.Kind == "habit" && len(after.Events) >= len(before.Events) {
		for _, v := range after.Events[len(before.Events):] {
			if v.Kind == "checkin" {
				parsed, _ := time.Parse("2006-01-02", v.Date)
				day := int(parsed.Weekday())
				if (after.Date == "" || after.Date <= v.Date) && slices.Contains(after.Weekdays, day) {
					add("habit", fmt.Sprintf("%d:%s", after.ID, v.Date), v.ActorID, v.Date, 5, 1)
				}
			}
		}
	}
	complete := func(e FamilyEntry) bool {
		if len(e.Steps) == 0 {
			return false
		}
		for i := range e.Steps {
			if !slices.ContainsFunc(e.Events, func(v FamilyEvent) bool { return v.Kind == "step" && v.Step == i }) {
				return false
			}
		}
		return true
	}
	if (after.Kind == "adventure" || (after.Kind == "proposal" && after.Approved)) && !complete(before) && complete(after) {
		ids := append([]int64{}, after.ParticipantIDs...)
		date := ""
		for _, v := range after.Events {
			if v.Kind == "step" {
				if len(after.ParticipantIDs) == 0 && !slices.Contains(ids, v.ActorID) {
					ids = append(ids, v.ActorID)
				}
				if v.Date > date {
					date = v.Date
				}
			}
		}
		for _, id := range ids {
			add("adventure", fmt.Sprint(after.ID), id, date, 20, 3)
		}
	}
	return out
}
