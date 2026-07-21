package store

import "time"

type Participant struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
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
	ID            int64      `json:"id"`
	AssignmentID  int64      `json:"assignmentId"`
	ChoreID       int64      `json:"choreId"`
	ParticipantID int64      `json:"participantId"`
	ChoreTitle    string     `json:"choreTitle"`
	PersonName    string     `json:"personName"`
	DueDate       string     `json:"dueDate"`
	TimeWindow    string     `json:"timeWindow"`
	BenefitType   string     `json:"benefitType"`
	ExecutionMode string     `json:"executionMode"`
	Status        string     `json:"status"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
	ConfirmedAt   *time.Time `json:"confirmedAt,omitempty"`
	AverageRating float64    `json:"averageRating"`
	Reward        float64    `json:"reward"`
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
