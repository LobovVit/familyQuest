package domain

import (
	"errors"
	"time"
)

const (
	RoleParent = "parent"
	RoleChild  = "child"
	RoleSchool = "school"
)

type Principal struct {
	ParticipantID int64
	Role          string
}

func (p Principal) IsParent() bool { return p.Role == RoleParent }
func ValidatePIN(pin string) error {
	if len(pin) != 6 {
		return ErrInvalidPINFormat
	}
	for _, r := range pin {
		if r < '0' || r > '9' {
			return ErrInvalidPINFormat
		}
	}
	return nil
}
func ValidateRole(role string) error {
	if role != RoleParent && role != RoleChild && role != RoleSchool {
		return ErrInvalidRole
	}
	return nil
}
func CanComplete(p Principal, ownerID int64) bool { return p.ParticipantID == ownerID }

var (
	ErrNotFound         = errors.New("not found")
	ErrInvalidRating    = errors.New("rating must be between 1 and 5")
	ErrInvalidPIN       = errors.New("invalid pin")
	ErrInvalidPINFormat = errors.New("pin must contain 6 digits")
	ErrInvalidRole      = errors.New("invalid role")
	ErrUnauthorized     = errors.New("authentication required")
	ErrForbidden        = errors.New("forbidden")
)

type Participant struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
}
type Chore struct {
	ID               int64     `json:"id"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	Schedule         string    `json:"schedule"`
	TimeWindow       string    `json:"timeWindow"`
	BenefitType      string    `json:"benefitType"`
	ExecutionMode    string    `json:"executionMode"`
	BaseValue        int       `json:"baseValue"`
	ParticipantIDs   []int64   `json:"participantIds"`
	ParticipantNames []string  `json:"participantNames"`
	Active           bool      `json:"active"`
	CreatedAt        time.Time `json:"createdAt"`
}
type Assignment struct {
	ID            int64     `json:"id"`
	ChoreID       int64     `json:"choreId"`
	ParticipantID int64     `json:"participantId"`
	ChoreTitle    string    `json:"choreTitle"`
	PersonName    string    `json:"personName"`
	Schedule      string    `json:"schedule"`
	TimeWindow    string    `json:"timeWindow"`
	BenefitType   string    `json:"benefitType"`
	ExecutionMode string    `json:"executionMode"`
	BaseValue     int       `json:"baseValue"`
	CreatedAt     time.Time `json:"createdAt"`
}
type Task struct {
	ID               int64      `json:"id"`
	AssignmentID     int64      `json:"assignmentId"`
	ChoreID          int64      `json:"choreId"`
	ParticipantID    int64      `json:"participantId"`
	ChoreTitle       string     `json:"choreTitle"`
	ChoreDescription string     `json:"choreDescription"`
	PersonName       string     `json:"personName"`
	DueDate          string     `json:"dueDate"`
	Schedule         string     `json:"schedule"`
	TimeWindow       string     `json:"timeWindow"`
	BenefitType      string     `json:"benefitType"`
	ExecutionMode    string     `json:"executionMode"`
	Status           string     `json:"status"`
	CompletedAt      *time.Time `json:"completedAt,omitempty"`
	ConfirmedAt      *time.Time `json:"confirmedAt,omitempty"`
	AverageRating    float64    `json:"averageRating"`
	Reward           float64    `json:"reward"`
}
type WeekPlanItem struct {
	AssignmentID   int64  `json:"assignmentId"`
	ChoreID        int64  `json:"choreId"`
	ParticipantID  int64  `json:"participantId"`
	ChoreTitle     string `json:"choreTitle"`
	PersonName     string `json:"personName"`
	Schedule       string `json:"schedule"`
	TimeWindow     string `json:"timeWindow"`
	BenefitType    string `json:"benefitType"`
	ExecutionMode  string `json:"executionMode"`
	PlannedCount   int    `json:"plannedCount"`
	DoneCount      int    `json:"doneCount"`
	ConfirmedCount int    `json:"confirmedCount"`
}
type LeaderboardEntry struct {
	ParticipantID  int64   `json:"participantId"`
	Name           string  `json:"name"`
	TasksDone      int     `json:"tasksDone"`
	TasksAssigned  int     `json:"tasksAssigned"`
	Reward         float64 `json:"reward"`
	AverageRating  float64 `json:"averageRating"`
	BehaviorRating float64 `json:"behaviorRating"`
	BehaviorCount  int     `json:"behaviorCount"`
	BehaviorSmiles int     `json:"behaviorSmiles"`
}
type BehaviorRating struct {
	ID                  int64     `json:"id"`
	RatedDate           string    `json:"ratedDate"`
	RaterParticipantID  int64     `json:"raterParticipantId"`
	TargetParticipantID int64     `json:"targetParticipantId"`
	Rating              int       `json:"rating"`
	Comment             string    `json:"comment"`
	CreatedAt           time.Time `json:"createdAt"`
}
type Reward struct {
	ID               int64     `json:"id"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	Period           string    `json:"period"`
	RewardType       string    `json:"rewardType"`
	StarCost         int       `json:"starCost"`
	SmileCost        int       `json:"smileCost"`
	ParticipantIDs   []int64   `json:"participantIds"`
	ParticipantNames []string  `json:"participantNames"`
	Active           bool      `json:"active"`
	CreatedAt        time.Time `json:"createdAt"`
}
