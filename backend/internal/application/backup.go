package application

import "time"

const BackupVersion = 1

type BackupData struct {
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
