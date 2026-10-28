package application

import (
	"context"
	"slices"
	"time"

	"github.com/lobov/familyquest/backend/internal/domain"
)

type MCPResults struct {
	Date  string                    `json:"date"`
	Tasks []domain.Task             `json:"tasks"`
	Day   []domain.LeaderboardEntry `json:"day"`
	Week  []domain.LeaderboardEntry `json:"week"`
	Month []domain.LeaderboardEntry `json:"month"`
}

func (s *Service) MCPResults(ctx context.Context, p domain.Principal, date string) (MCPResults, error) {
	out := MCPResults{Date: date}
	if (s.familyID > 0 && p.FamilyID != s.familyID) || p.ParticipantID <= 0 || domain.ValidateRole(p.Role) != nil {
		return out, domain.ErrForbidden
	}
	if !domain.ValidFamilyDate(date) {
		return out, domain.ErrInvalidInput
	}
	day, _ := time.Parse("2006-01-02", date)
	tasks, err := s.ListTasks(ctx, day)
	if err != nil {
		return out, err
	}
	out.Tasks = []domain.Task{}
	for _, t := range tasks {
		if p.Role != domain.RoleChild || t.ParticipantID == p.ParticipantID {
			out.Tasks = append(out.Tasks, t)
		}
	}
	for period, dst := range map[string]*[]domain.LeaderboardEntry{"day": &out.Day, "week": &out.Week, "month": &out.Month} {
		rows, err := s.Leaderboard(ctx, period, day)
		if err != nil {
			return out, err
		}
		*dst = []domain.LeaderboardEntry{}
		for _, v := range rows {
			if p.Role != domain.RoleChild || v.ParticipantID == p.ParticipantID {
				*dst = append(*dst, v)
			}
		}
	}
	return out, nil
}

type MCPActivity struct {
	ParticipantIDs []int64              `json:"participantIds"`
	Sport          *domain.SportSession `json:"sport,omitempty"`
	ID             int64                `json:"id"`
	Version        int                  `json:"version"`
	Kind           string               `json:"kind"`
	Title          string               `json:"title"`
	Date           string               `json:"date"`
	Steps          []domain.FamilyStep  `json:"steps"`
	Weekdays       []int                `json:"weekdays"`
	Progress       []domain.FamilyEvent `json:"progress"`
}

func (s *Service) MCPActivities(ctx context.Context, p domain.Principal) ([]MCPActivity, error) {
	entries, err := s.FamilyEntries(ctx, p)
	if err != nil {
		return nil, err
	}
	out := []MCPActivity{}
	for _, e := range entries {
		if e.Archived || (e.Kind == "proposal" && !e.Approved) || !slices.Contains([]string{"habit", "value", "skill", "adventure", "proposal", "sport"}, e.Kind) {
			continue
		}
		if !p.IsParent() && len(e.ParticipantIDs) > 0 && !slices.Contains(e.ParticipantIDs, p.ParticipantID) {
			continue
		}
		events := []domain.FamilyEvent{}
		for _, v := range e.Events {
			if !slices.Contains([]string{"checkin", "step", "stage"}, v.Kind) {
				continue
			}
			if p.IsParent() || v.ActorID == p.ParticipantID {
				v.Note = ""
				events = append(events, v)
			}
		}
		out = append(out, MCPActivity{ParticipantIDs: e.ParticipantIDs, Sport: e.Sport, ID: e.ID, Version: e.Version, Kind: e.Kind, Title: e.Title, Date: e.Date, Steps: e.Steps, Weekdays: e.Weekdays, Progress: events})
	}
	return out, nil
}
func (s *Service) MCPLearningResults(ctx context.Context, p domain.Principal, target int64) (map[string]any, error) {
	if (s.familyID > 0 && p.FamilyID != s.familyID) || !domain.IsFamilyMember(p) {
		return nil, domain.ErrForbidden
	}
	if target == 0 {
		target = p.ParticipantID
	}
	if !p.IsParent() && target != p.ParticipantID {
		return nil, domain.ErrForbidden
	}
	owner, err := s.repo.GetParticipant(ctx, target)
	if err != nil {
		return nil, err
	}
	if !owner.Active {
		return nil, domain.ErrForbidden
	}
	rewards, err := s.repo.ActivityRewards(ctx, target)
	if err != nil {
		return nil, err
	}
	sessions, err := s.repo.ListMathSessions(ctx, target)
	if err != nil {
		return nil, err
	}
	// Views may contain pending exercises and answers; expose only completed results.
	finished := []domain.MathView{}
	for _, v := range sessions {
		if v.View().Finished {
			finished = append(finished, v.View())
		}
	}
	return map[string]any{"participantId": target, "rewards": rewards, "math": finished}, nil
}
