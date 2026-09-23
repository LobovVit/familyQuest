package application

import (
	"fmt"
	"github.com/lobov/familyquest/backend/internal/domain"
	"time"
)

const BackupVersion = 4

type BackupData struct {
	MathSessions       []domain.MathSession      `json:"mathSessions"`
	ActivityRewards    []domain.ActivityReward   `json:"activityRewards"`
	FamilyEntries      []domain.FamilyEntry      `json:"familyEntries"`
	Version            int                       `json:"version"`
	ExportedAt         time.Time                 `json:"exportedAt"`
	Participants       []BackupParticipant       `json:"participants"`
	Chores             []BackupChore             `json:"chores"`
	Assignments        []BackupAssignment        `json:"assignments"`
	Tasks              []BackupTask              `json:"tasks"`
	Confirmations      []BackupConfirmation      `json:"confirmations"`
	BehaviorRatings    []BackupBehaviorRating    `json:"behaviorRatings"`
	Rewards            []BackupReward            `json:"rewards"`
	RewardParticipants []BackupRewardParticipant `json:"rewardParticipants"`
}
type BackupParticipant struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	PINCode   string    `json:"pinCode,omitempty"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
}
type BackupChore struct {
	ID            int64     `json:"id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Schedule      string    `json:"schedule"`
	TimeWindow    string    `json:"timeWindow"`
	BenefitType   string    `json:"benefitType"`
	ExecutionMode string    `json:"executionMode"`
	BaseValue     int       `json:"baseValue"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"createdAt"`
}
type BackupAssignment struct {
	ID            int64     `json:"id"`
	ChoreID       int64     `json:"choreId"`
	ParticipantID int64     `json:"participantId"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"createdAt"`
}
type BackupTask struct {
	ID           int64      `json:"id"`
	AssignmentID int64      `json:"assignmentId"`
	DueDate      string     `json:"dueDate"`
	Status       string     `json:"status"`
	CompletedBy  *int64     `json:"completedBy,omitempty"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
	ConfirmedAt  *time.Time `json:"confirmedAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
}
type BackupConfirmation struct {
	ID            int64     `json:"id"`
	TaskID        int64     `json:"taskId"`
	ParticipantID int64     `json:"participantId"`
	Rating        int       `json:"rating"`
	Comment       string    `json:"comment"`
	CreatedAt     time.Time `json:"createdAt"`
}
type BackupBehaviorRating struct {
	ID                  int64     `json:"id"`
	RatedDate           string    `json:"ratedDate"`
	RaterParticipantID  int64     `json:"raterParticipantId"`
	TargetParticipantID int64     `json:"targetParticipantId"`
	Rating              int       `json:"rating"`
	Comment             string    `json:"comment"`
	CreatedAt           time.Time `json:"createdAt"`
}
type BackupReward struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Period      string    `json:"period"`
	RewardType  string    `json:"rewardType"`
	StarCost    int       `json:"starCost"`
	SmileCost   int       `json:"smileCost"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"createdAt"`
}
type BackupRewardParticipant struct {
	ID            int64     `json:"id"`
	RewardID      int64     `json:"rewardId"`
	ParticipantID int64     `json:"participantId"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"createdAt"`
}

// Validate rejects unsupported and unusable backups before touching stored data.
func (b BackupData) Validate() error {
	if b.Version != 1 && b.Version != 2 && b.Version != 3 && b.Version != BackupVersion {
		return fmt.Errorf("%w: unsupported backup version %d", domain.ErrInvalidInput, b.Version)
	}
	hasParent := false
	for _, p := range b.Participants {
		if p.ID <= 0 || p.Name == "" {
			return domain.ErrInvalidInput
		}
		if err := domain.ValidateRole(p.Role); err != nil {
			return err
		}
		if p.PINCode != "" {
			if err := domain.ValidatePIN(p.PINCode); err != nil {
				return err
			}
		}
		hasParent = hasParent || (p.Active && p.Role == domain.RoleParent)
	}
	if !hasParent {
		return fmt.Errorf("%w: backup requires an active parent", domain.ErrInvalidInput)
	}
	if err := b.validateFamily(); err != nil {
		return err
	}
	return b.validateLearning()
}
