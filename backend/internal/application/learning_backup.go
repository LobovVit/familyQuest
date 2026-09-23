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
		case "sport":
			if r.SourceKey != r.Date || r.Stars != 10 || r.Smiles != 2 {
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
			if err != nil || strconv.FormatInt(id, 10) != r.SourceKey || (kind != "adventure" && kind != "proposal") || r.Stars != 20 || r.Smiles != 3 {
				return domain.ErrInvalidInput
			}
		default:
			return domain.ErrInvalidInput
		}
	}
	for _, s := range b.MathSessions {
		for _, a := range s.Answers {
			if a.Correct && !seen[fmt.Sprintf("math/%s:%d/%d", s.ID, a.Index, s.ParticipantID)] {
				return domain.ErrInvalidInput
			}
		}
	}
	return nil
}
