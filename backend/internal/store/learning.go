package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/lobov/familyquest/backend/internal/domain"
	"time"
)

func insertActivityReward(ctx context.Context, tx pgx.Tx, r domain.ActivityReward) error {
	_, err := tx.Exec(ctx, `insert into activity_rewards(source,source_key,participant_id,earned_date,stars,smiles,title) select $1,$2,p.id,$4::date,$5,$6,$7 from participants p where p.id=$3 and p.active and p.role in ('parent','child') on conflict do nothing`, r.Source, r.SourceKey, r.ParticipantID, r.Date, r.Stars, r.Smiles, r.Title)
	return err
}
func (s *Store) CreateMathSession(ctx context.Context, session domain.MathSession) (domain.MathSession, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return session, err
	}
	defer tx.Rollback(ctx)
	// One active session per child, even when several iPads start simultaneously.
	var participantID int64
	if err = tx.QueryRow(ctx, `select id from participants where id=$1 and active and role='child' for update`, session.ParticipantID).Scan(&participantID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = domain.ErrForbidden
		}
		return session, err
	}
	var raw []byte
	err = tx.QueryRow(ctx, `select data from math_sessions where participant_id=$1 and coalesce((data->>'closed')::boolean,false)=false and jsonb_array_length(data->'answers')<jsonb_array_length(data->'questions') order by created_at desc limit 1`, session.ParticipantID).Scan(&raw)
	if err == nil {
		err = json.Unmarshal(raw, &session)
		return session, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return session, err
	}
	raw, err = json.Marshal(session)
	if err != nil {
		return session, err
	}
	_, err = tx.Exec(ctx, `insert into math_sessions(id,participant_id,data,created_at) values($1,$2,$3,$4)`, session.ID, session.ParticipantID, raw, session.CreatedAt)
	if err != nil {
		return session, err
	}
	return session, tx.Commit(ctx)
}
func (s *Store) ListMathSessions(ctx context.Context, id int64) ([]domain.MathSession, error) {
	rows, err := s.pool.Query(ctx, `select data from math_sessions where participant_id=$1 order by created_at desc limit 100`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.MathSession{}
	for rows.Next() {
		var raw []byte
		var session domain.MathSession
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &session); err != nil {
			return nil, err
		}
		out = append(out, session)
	}
	return out, rows.Err()
}
func (s *Store) AnswerMath(ctx context.Context, owner int64, id string, index int, values map[string]int, now time.Time) (domain.MathSession, error) {
	var session domain.MathSession
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return session, err
	}
	defer tx.Rollback(ctx)
	var participantID int64
	if err = tx.QueryRow(ctx, `select id from participants where id=$1 and active and role='child' for update`, owner).Scan(&participantID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = domain.ErrForbidden
		}
		return session, err
	}
	var raw []byte
	err = tx.QueryRow(ctx, `select data from math_sessions where id=$1 and participant_id=$2 for update`, id, owner).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return session, domain.ErrNotFound
	}
	if err != nil {
		return session, err
	}
	if err = json.Unmarshal(raw, &session); err != nil {
		return session, err
	}
	count := len(session.Answers)
	if err = session.Submit(index, values, now); err != nil {
		return session, err
	}
	if len(session.Answers) > count {
		a := session.Answers[index]
		if session.RewardVersion == 2 && a.Stars > 0 {
			var earned int
			if err = tx.QueryRow(ctx, `select coalesce(sum(stars),0)::int from activity_rewards where participant_id=$1 and source='math' and earned_date=$2::date`, owner, a.Date).Scan(&earned); err != nil {
				return session, err
			}
			a.Stars = min(a.Stars, max(0, domain.MathDailyStarLimit-earned))
			session.Answers[index] = a
		}
		if a.Stars > 0 {
			err = insertActivityReward(ctx, tx, domain.ActivityReward{Source: "math", SourceKey: fmt.Sprintf("%s:%d", id, index), ParticipantID: owner, Date: a.Date, Stars: a.Stars, Title: "Математика " + session.Settings.Operation})
			if err != nil {
				return session, err
			}
		}
		raw, err = json.Marshal(session)
		if err != nil {
			return session, err
		}
		if _, err = tx.Exec(ctx, `update math_sessions set data=$2 where id=$1`, id, raw); err != nil {
			return session, err
		}
	}
	return session, tx.Commit(ctx)
}
func (s *Store) ActivityRewards(ctx context.Context, id int64) ([]domain.ActivityReward, error) {
	rows, err := s.pool.Query(ctx, `select source,source_key,participant_id,earned_date::text,stars,smiles,title from activity_rewards where participant_id=$1 order by earned_date desc,source,source_key limit 200`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ActivityReward{}
	for rows.Next() {
		var r domain.ActivityReward
		if err = rows.Scan(&r.Source, &r.SourceKey, &r.ParticipantID, &r.Date, &r.Stars, &r.Smiles, &r.Title); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) FinishMath(ctx context.Context, owner int64, id string) (domain.MathSession, error) {
	var session domain.MathSession
	var raw []byte
	err := s.pool.QueryRow(ctx, `update math_sessions set data=jsonb_set(data,'{closed}','true') where id=$1 and participant_id=$2 returning data`, id, owner).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return session, domain.ErrNotFound
	}
	if err != nil {
		return session, err
	}
	err = json.Unmarshal(raw, &session)
	return session, err
}
