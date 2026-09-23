package application

import (
	"context"
	"github.com/lobov/familyquest/backend/internal/domain"
	"slices"
	"time"
)

// FamilyOverview exposes only the fields used by the read-only dashboard.
// Never embed FamilyEntry: notes, media and private discussions stay authenticated.
type FamilyOverview struct {
	Date       string              `json:"date"`
	WeekStart  string              `json:"weekStart"`
	Sports     []OverviewSport     `json:"sports"`
	Habits     []OverviewHabit     `json:"habits"`
	Adventures []OverviewAdventure `json:"adventures"`
}
type OverviewCard struct {
	ID             int64   `json:"id"`
	Title          string  `json:"title"`
	ParticipantIDs []int64 `json:"participantIds"`
}
type OverviewSport struct {
	OverviewCard
	Date       string  `json:"date"`
	Activity   string  `json:"activity"`
	Minutes    float64 `json:"minutes"`
	DistanceKm float64 `json:"distanceKm"`
}
type OverviewHabit struct {
	OverviewCard
	CompletedIDs []int64 `json:"completedIds"`
}
type OverviewAdventure struct {
	OverviewCard
	Date           string `json:"date"`
	CompletedSteps int    `json:"completedSteps"`
	TotalSteps     int    `json:"totalSteps"`
}

func (s *Service) FamilyOverview(ctx context.Context, date string) (FamilyOverview, error) {
	out := FamilyOverview{Date: date, Sports: []OverviewSport{}, Habits: []OverviewHabit{}, Adventures: []OverviewAdventure{}}
	if !domain.ValidFamilyDate(date) {
		return out, domain.ErrInvalidInput
	}
	day, _ := time.Parse("2006-01-02", date)
	out.WeekStart = day.AddDate(0, 0, -(int(day.Weekday())+6)%7).Format("2006-01-02")
	entries, err := s.repo.ListFamilyEntries(ctx)
	if err != nil {
		return out, err
	}
	for _, e := range entries {
		if e.Archived {
			continue
		}
		card := OverviewCard{ID: e.ID, Title: e.Title, ParticipantIDs: append([]int64{}, e.ParticipantIDs...)}
		switch e.Kind {
		case "sport":
			if e.Sport != nil && e.Date >= out.WeekStart && e.Date <= date {
				out.Sports = append(out.Sports, OverviewSport{card, e.Date, e.Sport.Activity, e.Sport.Minutes, e.Sport.DistanceKm})
			}
		case "habit":
			if e.Date > date || !slices.Contains(e.Weekdays, int(day.Weekday())) {
				continue
			}
			completed := []int64{}
			for _, v := range e.Events {
				if v.Kind == "checkin" && v.Date == date && !slices.Contains(completed, v.ActorID) {
					completed = append(completed, v.ActorID)
				}
			}
			out.Habits = append(out.Habits, OverviewHabit{card, completed})
		case "adventure", "proposal":
			if e.Kind == "proposal" && !e.Approved {
				continue
			}
			done := map[int]bool{}
			for _, v := range e.Events {
				if v.Kind == "step" && v.Date <= date && v.Step >= 0 && v.Step < len(e.Steps) {
					done[v.Step] = true
				}
			}
			out.Adventures = append(out.Adventures, OverviewAdventure{card, e.Date, len(done), len(e.Steps)})
		}
	}
	slices.SortFunc(out.Sports, func(a, b OverviewSport) int {
		if a.Date > b.Date {
			return -1
		}
		if a.Date < b.Date {
			return 1
		}
		return 0
	})
	slices.SortFunc(out.Adventures, func(a, b OverviewAdventure) int {
		if a.Date < b.Date {
			return -1
		}
		if a.Date > b.Date {
			return 1
		}
		return 0
	})
	return out, nil
}
