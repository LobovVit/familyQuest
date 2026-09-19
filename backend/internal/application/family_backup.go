package application

import "github.com/lobov/familyquest/backend/internal/domain"

func (b BackupData) validateFamily() error {
	participants := map[int64]bool{}
	for _, p := range b.Participants {
		participants[p.ID] = true
	}
	ids := map[int64]bool{}
	for _, e := range b.FamilyEntries {
		if e.ID <= 0 || e.Version <= 0 || ids[e.ID] || !participants[e.AuthorID] || e.CreatedAt.IsZero() || e.UpdatedAt.IsZero() {
			return domain.ErrInvalidInput
		}
		ids[e.ID] = true
		if err := e.Validate(); err != nil {
			return err
		}
		for _, id := range e.ParticipantIDs {
			if !participants[id] {
				return domain.ErrInvalidInput
			}
		}
		for _, step := range e.Steps {
			if step.ParticipantID != 0 && !participants[step.ParticipantID] {
				return domain.ErrInvalidInput
			}
		}
		for _, ev := range e.Events {
			if !participants[ev.ActorID] {
				return domain.ErrInvalidInput
			}
		}
	}
	return nil
}
