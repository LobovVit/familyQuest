package store

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/lobov/familyquest/backend/internal/domain"
	"time"
)

func (s *Store) LearningProfile(ctx context.Context, id int64) (domain.LearningProfile, error) {
	var p domain.LearningProfile
	e := s.db.QueryRow(ctx, `select coalesce(birth_date::text,''),math_level,reading_level from participants where id=$1 and active and role='child'`, id).Scan(&p.BirthDate, &p.MathLevel, &p.ReadingLevel)
	if errors.Is(e, pgx.ErrNoRows) {
		e = domain.ErrNotFound
	}
	return p, e
}
func (s *Store) SaveLearningProfile(ctx context.Context, id int64, p domain.LearningProfile) error {
	if e := p.Validate(time.Now()); e != nil {
		return e
	}
	tag, e := s.db.Exec(ctx, `update participants set birth_date=nullif($2,'')::date,math_level=$3,reading_level=$4 where id=$1 and active and role='child'`, id, p.BirthDate, p.MathLevel, p.ReadingLevel)
	if e == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return e
}

// CheckWriteAccess evaluates expiry at use time, without relying on a scheduler.
// CheckWriteAccess проверяет срок при операции, независимо от планировщика.
func (s *Store) CheckWriteAccess(ctx context.Context) error {
	var allowed bool
	e := s.db.QueryRow(ctx, `select access_until is null or access_until>now() from service_policy where id`).Scan(&allowed)
	if e != nil {
		return e
	}
	if !allowed {
		return domain.ErrForbidden
	}
	return nil
}
