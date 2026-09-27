package application

import (
	"fmt"
	"github.com/lobov/familyquest/backend/internal/domain"
	"strconv"
	"strings"
)

func (b BackupData) validateLearning() error {
	if b.Version < 3 && (len(b.MathSessions) > 0 || len(b.ActivityRewards) > 0) {
		return domain.ErrInvalidInput
	}
	people := map[int64]string{}
	for _, p := range b.Participants {
		people[p.ID] = p.Role
	}
	sessions := map[string]domain.MathSession{}
	activeOwners := map[int64]bool{}
	for _, s := range b.MathSessions {
		if _, exists := sessions[s.ID]; exists || people[s.ParticipantID] != domain.RoleChild {
			return domain.ErrInvalidInput
		}
		if err := s.Validate(); err != nil {
			return err
		}
		if !s.Closed && len(s.Answers) < len(s.Questions) {
			if activeOwners[s.ParticipantID] {
				return domain.ErrInvalidInput
			}
			activeOwners[s.ParticipantID] = true
		}
		if (b.Version < 4 && s.RewardVersion != 0) || (b.Version < 5 && s.RewardVersion == 3) {
			return domain.ErrInvalidInput
		}
		sessions[s.ID] = s
	}
	entries := map[int64]domain.FamilyEntry{}
	for _, e := range b.FamilyEntries {
		entries[e.ID] = e
	}
	seen := map[string]bool{}
	for _, r := range b.ActivityRewards {
		key := fmt.Sprintf("%s/%s/%d", r.Source, r.SourceKey, r.ParticipantID)
		if seen[key] || (people[r.ParticipantID] != domain.RoleChild && people[r.ParticipantID] != domain.RoleParent) || !domain.ValidFamilyDate(r.Date) || len([]rune(r.Title)) > 160 {
			return domain.ErrInvalidInput
		}
		seen[key] = true
		switch r.Source {
		case "math":
			parts := strings.Split(r.SourceKey, ":")
			if len(parts) != 2 {
				return domain.ErrInvalidInput
			}
			s, ok := sessions[parts[0]]
			index, err := strconv.Atoi(parts[1])
			if !ok || err != nil || index < 0 || index >= len(s.Answers) || s.ParticipantID != r.ParticipantID {
				return domain.ErrInvalidInput
			}
			a := s.Answers[index]
			if !a.Correct || r.Stars != a.Stars || r.Smiles != 0 || r.Date != a.Date {
				return domain.ErrInvalidInput
			}
		case "reading":
			valid := false
			for _, level := range []string{"phrases", "sentences", "advanced"} {
				c := domain.ReadingCompletion{ID: r.SourceKey, Level: level}
				if c.Validate() == nil && r.Title == c.Title() && r.Stars >= 0 && r.Stars <= c.Stars() {
					valid = true
				}
			}
			if b.Version < 5 || people[r.ParticipantID] != domain.RoleChild || !valid || r.Smiles != 0 {
				return domain.ErrInvalidInput
			}
		case "sport":
			if r.SourceKey != r.Date || (r.Stars != 10 && (b.Version < 4 || r.Stars != 30)) || r.Smiles != 2 {
				return domain.ErrInvalidInput
			}
		case "habit":
			parts := strings.Split(r.SourceKey, ":")
			if len(parts) != 2 || parts[1] != r.Date {
				return domain.ErrInvalidInput
			}
			id, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil || strconv.FormatInt(id, 10) != parts[0] || entries[id].Kind != "habit" || r.Stars != 5 || r.Smiles != 1 {
				return domain.ErrInvalidInput
			}
		case "adventure":
			id, err := strconv.ParseInt(r.SourceKey, 10, 64)
			kind := entries[id].Kind
			if err != nil || strconv.FormatInt(id, 10) != r.SourceKey || (kind != "adventure" && kind != "proposal") || (r.Stars != 20 && (b.Version < 4 || r.Stars != 40)) || r.Smiles != 3 {
				return domain.ErrInvalidInput
			}
		default:
			return domain.ErrInvalidInput
		}
	}
	for _, s := range b.MathSessions {
		for _, a := range s.Answers {
			if a.Stars > 0 && !seen[fmt.Sprintf("math/%s:%d/%d", s.ID, a.Index, s.ParticipantID)] {
				return domain.ErrInvalidInput
			}
		}
	}
	// Check the new daily budget across sessions, including sessions resumed after midnight.
	daily := map[string]int{}
	previousDaily := map[string]int{}
	for _, session := range b.MathSessions {
		if session.RewardVersion < 2 {
			continue
		}
		for _, a := range session.Answers {
			key := fmt.Sprintf("%d/%s", session.ParticipantID, a.Date)
			daily[key] += a.Stars
			if session.RewardVersion == 2 {
				previousDaily[key] += a.Stars
				if previousDaily[key] > domain.PreviousMathDailyStarLimit {
					return domain.ErrInvalidInput
				}
			}
			if daily[key] > domain.MathDailyStarLimit {
				return domain.ErrInvalidInput
			}
		}
	}
	readingDaily := map[string]int{}
	for _, r := range b.ActivityRewards {
		if r.Source != "reading" {
			continue
		}
		key := fmt.Sprintf("%d/%s", r.ParticipantID, r.Date)
		readingDaily[key] += r.Stars
		if readingDaily[key] > domain.ReadingDailyStarLimit {
			return domain.ErrInvalidInput
		}
	}
	return nil
}
