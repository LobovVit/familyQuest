package store

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/lobov/familyquest/backend/internal/domain"
	"time"
)

func (s *Store) CompleteReading(ctx context.Context, owner int64, completion domain.ReadingCompletion, now time.Time) (domain.ActivityReward, error) {
	reward := completion.Reward(owner, 0, now)
	if err := completion.Validate(); err != nil {
		return reward, err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return reward, err
	}
	defer tx.Rollback(ctx)
	var participantID int64
	if err = tx.QueryRow(ctx, `select id from participants where id=$1 and active and role='child' for update`, owner).Scan(&participantID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = domain.ErrForbidden
		}
		return reward, err
	}
	// Replay before checking today's budget, including retries across midnight.
	// Повтор возвращает исходную награду, в том числе после полуночи.
	var saved domain.ActivityReward
	err = tx.QueryRow(ctx, `select source,source_key,participant_id,earned_date::text,stars,smiles,title from activity_rewards where source='reading' and source_key=$1 and participant_id=$2`, completion.ID, owner).Scan(&saved.Source, &saved.SourceKey, &saved.ParticipantID, &saved.Date, &saved.Stars, &saved.Smiles, &saved.Title)
	if err == nil {
		if saved.Title != completion.Title() {
			return reward, domain.ErrConflict
		}
		return saved, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return reward, err
	}
	if s.familyID > 0 {
		var allowed bool
		if err = tx.QueryRow(ctx, `select exists(select 1 from reading_starts where id=$1 and participant_id=$2 and level=$3)`, completion.ID, owner, completion.Level).Scan(&allowed); err != nil {
			return reward, err
		}
		if !allowed {
			return reward, domain.ErrForbidden
		}
	}

	var earned int
	if err = tx.QueryRow(ctx, `select coalesce(sum(stars),0)::int from activity_rewards where participant_id=$1 and source='reading' and earned_date=$2::date`, owner, reward.Date).Scan(&earned); err != nil {
		return reward, err
	}
	reward = completion.Reward(owner, earned, now)
	// Persist zero too: a retry tomorrow must not turn this completion into a new reward.
	// Сохраняем и ноль: повтор завтра не должен начислять новую награду.
	if err = insertActivityReward(ctx, tx, reward); err != nil {
		return reward, err
	}
	if _, err = tx.Exec(ctx, `delete from reading_starts where id=$1 and participant_id=$2`, completion.ID, owner); err != nil {
		return reward, err
	}
	return reward, tx.Commit(ctx)
}

// StartReading snapshots the approved level before the child starts reading.
// StartReading сохраняет разрешённый уровень до начала чтения.
func (s *Store) StartReading(ctx context.Context, owner int64, c domain.ReadingCompletion, now time.Time) error {
	if e := c.Validate(); e != nil {
		return e
	}
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer rollback(tx)
	var p domain.LearningProfile
	if e = tx.QueryRow(ctx, `select coalesce(birth_date::text,''),math_level,reading_level from participants where id=$1 and active and role='child' for update`, owner).Scan(&p.BirthDate, &p.MathLevel, &p.ReadingLevel); e != nil {
		return e
	}
	var priorOwner int64
	var priorLevel string
	e = tx.QueryRow(ctx, `select participant_id,level from reading_starts where id=$1`, c.ID).Scan(&priorOwner, &priorLevel)
	if e == nil {
		if priorOwner != owner || priorLevel != c.Level {
			return domain.ErrConflict
		}
		return nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	policy := p.Policy(now)
	if s.familyID > 0 && !policy.AllowsReading(c.Level) {
		return domain.ErrForbidden
	}
	if _, e = tx.Exec(ctx, `delete from reading_starts where participant_id=$1 and created_at<$2`, owner, now.Add(-24*time.Hour)); e != nil {
		return e
	}
	var count int
	if e = tx.QueryRow(ctx, `select count(*) from reading_starts where participant_id=$1`, owner).Scan(&count); e != nil {
		return e
	}
	if count >= 20 {
		return domain.ErrConflict
	}
	if _, e = tx.Exec(ctx, `insert into reading_starts(id,participant_id,level,policy_version,created_at) values($1,$2,$3,$4,$5)`, c.ID, owner, c.Level, policy.Version, now); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
