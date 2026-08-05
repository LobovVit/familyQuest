package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
)

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
	PINCode   string    `json:"pinCode"`
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

func (s *Store) ExportBackup(ctx context.Context) (BackupData, error) {
	backup := emptyBackupData()
	if err := s.scanBackupRows(ctx, &backup); err != nil {
		return BackupData{}, err
	}
	return backup, nil
}

func emptyBackupData() BackupData {
	return BackupData{
		Version:            BackupVersion,
		ExportedAt:         time.Now().UTC(),
		Participants:       []BackupParticipant{},
		Chores:             []BackupChore{},
		Assignments:        []BackupAssignment{},
		Tasks:              []BackupTask{},
		Confirmations:      []BackupConfirmation{},
		BehaviorRatings:    []BackupBehaviorRating{},
		Rewards:            []BackupReward{},
		RewardParticipants: []BackupRewardParticipant{},
	}
}

func (s *Store) ImportBackup(ctx context.Context, backup BackupData) error {
	if backup.Version != BackupVersion {
		return fmt.Errorf("unsupported backup version %d", backup.Version)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `truncate reward_participants, rewards, behavior_ratings, confirmations, tasks, assignments, chores, participants restart identity cascade`); err != nil {
		return err
	}
	for _, item := range backup.Participants {
		if _, err := tx.Exec(ctx, `
			insert into participants (id, name, role, pin_code, active, created_at)
			overriding system value values ($1, $2, $3, $4, $5, $6)
		`, item.ID, item.Name, item.Role, item.PINCode, item.Active, item.CreatedAt); err != nil {
			return err
		}
	}
	for _, item := range backup.Chores {
		if _, err := tx.Exec(ctx, `
			insert into chores (id, title, description, schedule, time_window, benefit_type, execution_mode, base_value, active, created_at)
			overriding system value values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, item.ID, item.Title, item.Description, item.Schedule, item.TimeWindow, item.BenefitType, item.ExecutionMode, item.BaseValue, item.Active, item.CreatedAt); err != nil {
			return err
		}
	}
	for _, item := range backup.Assignments {
		if _, err := tx.Exec(ctx, `
			insert into assignments (id, chore_id, participant_id, active, created_at)
			overriding system value values ($1, $2, $3, $4, $5)
		`, item.ID, item.ChoreID, item.ParticipantID, item.Active, item.CreatedAt); err != nil {
			return err
		}
	}
	for _, item := range backup.Tasks {
		if _, err := tx.Exec(ctx, `
			insert into tasks (id, assignment_id, due_date, status, completed_by, completed_at, confirmed_at, created_at)
			overriding system value values ($1, $2, $3::date, $4, $5, $6, $7, $8)
		`, item.ID, item.AssignmentID, item.DueDate, item.Status, item.CompletedBy, item.CompletedAt, item.ConfirmedAt, item.CreatedAt); err != nil {
			return err
		}
	}
	for _, item := range backup.Confirmations {
		if _, err := tx.Exec(ctx, `
			insert into confirmations (id, task_id, participant_id, rating, comment, created_at)
			overriding system value values ($1, $2, $3, $4, $5, $6)
		`, item.ID, item.TaskID, item.ParticipantID, item.Rating, item.Comment, item.CreatedAt); err != nil {
			return err
		}
	}
	for _, item := range backup.BehaviorRatings {
		if _, err := tx.Exec(ctx, `
			insert into behavior_ratings (id, rated_date, rater_participant_id, target_participant_id, rating, comment, created_at)
			overriding system value values ($1, $2::date, $3, $4, $5, $6, $7)
		`, item.ID, item.RatedDate, item.RaterParticipantID, item.TargetParticipantID, item.Rating, item.Comment, item.CreatedAt); err != nil {
			return err
		}
	}
	for _, item := range backup.Rewards {
		if _, err := tx.Exec(ctx, `
			insert into rewards (id, title, description, period, reward_type, star_cost, smile_cost, active, created_at)
			overriding system value values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, item.ID, item.Title, item.Description, item.Period, item.RewardType, item.StarCost, item.SmileCost, item.Active, item.CreatedAt); err != nil {
			return err
		}
	}
	for _, item := range backup.RewardParticipants {
		if _, err := tx.Exec(ctx, `
			insert into reward_participants (id, reward_id, participant_id, active, created_at)
			overriding system value values ($1, $2, $3, $4, $5)
		`, item.ID, item.RewardID, item.ParticipantID, item.Active, item.CreatedAt); err != nil {
			return err
		}
	}
	if err := resetSequences(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) SeedFromBackupFile(ctx context.Context, path string) (bool, error) {
	if path == "" {
		return false, nil
	}
	payload, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var backup BackupData
	if err := json.Unmarshal(payload, &backup); err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	if err := s.ImportBackup(ctx, backup); err != nil {
		return false, err
	}
	return true, nil
}

func ResolveSeedPath(path string) string {
	if path != "" {
		return path
	}
	candidates := []string{
		filepath.Join("seed", "familyquest-backup.json"),
		filepath.Join("backend", "seed", "familyquest-backup.json"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}

func (s *Store) HasAnyData(ctx context.Context) (bool, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		select
			(select count(*) from participants) +
			(select count(*) from chores) +
			(select count(*) from assignments) +
			(select count(*) from tasks) +
			(select count(*) from confirmations) +
			(select count(*) from behavior_ratings) +
			(select count(*) from rewards) +
			(select count(*) from reward_participants)
	`).Scan(&count)
	return count > 0, err
}

func (s *Store) scanBackupRows(ctx context.Context, backup *BackupData) error {
	if err := scanRows(ctx, s.pool.Query, `select id, name, role, pin_code, active, created_at from participants order by id`, func(rows pgx.Rows) error {
		var item BackupParticipant
		if err := rows.Scan(&item.ID, &item.Name, &item.Role, &item.PINCode, &item.Active, &item.CreatedAt); err != nil {
			return err
		}
		backup.Participants = append(backup.Participants, item)
		return nil
	}); err != nil {
		return err
	}
	if err := scanRows(ctx, s.pool.Query, `select id, title, description, schedule, time_window, benefit_type, execution_mode, base_value, active, created_at from chores order by id`, func(rows pgx.Rows) error {
		var item BackupChore
		if err := rows.Scan(&item.ID, &item.Title, &item.Description, &item.Schedule, &item.TimeWindow, &item.BenefitType, &item.ExecutionMode, &item.BaseValue, &item.Active, &item.CreatedAt); err != nil {
			return err
		}
		backup.Chores = append(backup.Chores, item)
		return nil
	}); err != nil {
		return err
	}
	if err := scanRows(ctx, s.pool.Query, `select id, chore_id, participant_id, active, created_at from assignments order by id`, func(rows pgx.Rows) error {
		var item BackupAssignment
		if err := rows.Scan(&item.ID, &item.ChoreID, &item.ParticipantID, &item.Active, &item.CreatedAt); err != nil {
			return err
		}
		backup.Assignments = append(backup.Assignments, item)
		return nil
	}); err != nil {
		return err
	}
	if err := scanRows(ctx, s.pool.Query, `select id, assignment_id, due_date::text, status, completed_by, completed_at, confirmed_at, created_at from tasks order by id`, func(rows pgx.Rows) error {
		var item BackupTask
		if err := rows.Scan(&item.ID, &item.AssignmentID, &item.DueDate, &item.Status, &item.CompletedBy, &item.CompletedAt, &item.ConfirmedAt, &item.CreatedAt); err != nil {
			return err
		}
		backup.Tasks = append(backup.Tasks, item)
		return nil
	}); err != nil {
		return err
	}
	if err := scanRows(ctx, s.pool.Query, `select id, task_id, participant_id, rating, comment, created_at from confirmations order by id`, func(rows pgx.Rows) error {
		var item BackupConfirmation
		if err := rows.Scan(&item.ID, &item.TaskID, &item.ParticipantID, &item.Rating, &item.Comment, &item.CreatedAt); err != nil {
			return err
		}
		backup.Confirmations = append(backup.Confirmations, item)
		return nil
	}); err != nil {
		return err
	}
	if err := scanRows(ctx, s.pool.Query, `select id, rated_date::text, rater_participant_id, target_participant_id, rating, comment, created_at from behavior_ratings order by id`, func(rows pgx.Rows) error {
		var item BackupBehaviorRating
		if err := rows.Scan(&item.ID, &item.RatedDate, &item.RaterParticipantID, &item.TargetParticipantID, &item.Rating, &item.Comment, &item.CreatedAt); err != nil {
			return err
		}
		backup.BehaviorRatings = append(backup.BehaviorRatings, item)
		return nil
	}); err != nil {
		return err
	}
	if err := scanRows(ctx, s.pool.Query, `select id, title, description, period, reward_type, star_cost, smile_cost, active, created_at from rewards order by id`, func(rows pgx.Rows) error {
		var item BackupReward
		if err := rows.Scan(&item.ID, &item.Title, &item.Description, &item.Period, &item.RewardType, &item.StarCost, &item.SmileCost, &item.Active, &item.CreatedAt); err != nil {
			return err
		}
		backup.Rewards = append(backup.Rewards, item)
		return nil
	}); err != nil {
		return err
	}
	return scanRows(ctx, s.pool.Query, `select id, reward_id, participant_id, active, created_at from reward_participants order by id`, func(rows pgx.Rows) error {
		var item BackupRewardParticipant
		if err := rows.Scan(&item.ID, &item.RewardID, &item.ParticipantID, &item.Active, &item.CreatedAt); err != nil {
			return err
		}
		backup.RewardParticipants = append(backup.RewardParticipants, item)
		return nil
	})
}

func scanRows(ctx context.Context, query func(context.Context, string, ...any) (pgx.Rows, error), sql string, scan func(pgx.Rows) error) error {
	rows, err := query(ctx, sql)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

func resetSequences(ctx context.Context, tx pgx.Tx) error {
	tables := []string{"participants", "chores", "assignments", "tasks", "confirmations", "behavior_ratings", "rewards", "reward_participants"}
	for _, table := range tables {
		sql := fmt.Sprintf(`
			select setval(
				pg_get_serial_sequence('%[1]s', 'id'),
				greatest(coalesce((select max(id) from %[1]s), 0), 1),
				(select count(*) > 0 from %[1]s)
			)
		`, table)
		if _, err := tx.Exec(ctx, sql); err != nil {
			return err
		}
	}
	return nil
}
