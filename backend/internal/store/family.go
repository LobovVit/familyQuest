package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/lobov/familyquest/backend/internal/domain"
	"time"
)

func scanFamilyEntry(row pgx.Row) (domain.FamilyEntry, error) {
	var e domain.FamilyEntry
	var id, author int64
	var version int
	var created, updated time.Time
	var data []byte
	err := row.Scan(&id, &version, &author, &created, &updated, &data)
	if errors.Is(err, pgx.ErrNoRows) {
		return e, domain.ErrNotFound
	}
	if err != nil {
		return e, err
	}
	if err = json.Unmarshal(data, &e); err != nil {
		return e, err
	}
	e.ID = id
	e.Version = version
	e.AuthorID = author
	e.CreatedAt = created
	e.UpdatedAt = updated
	if e.Events == nil {
		e.Events = []domain.FamilyEvent{}
	}
	return e, nil
}

const familyColumns = "id, version, author_id, created_at, updated_at, data"

func (s *Store) ListFamilyEntries(ctx context.Context) ([]domain.FamilyEntry, error) {
	rows, err := s.pool.Query(ctx, "select "+familyColumns+" from family_entries order by id desc")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.FamilyEntry{}
	for rows.Next() {
		e, err := scanFamilyEntry(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
func (s *Store) GetFamilyEntry(ctx context.Context, id int64) (domain.FamilyEntry, error) {
	return scanFamilyEntry(s.pool.QueryRow(ctx, "select "+familyColumns+" from family_entries where id=$1", id))
}
func (s *Store) SaveFamilyEntry(ctx context.Context, e domain.FamilyEntry) (domain.FamilyEntry, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return e, err
	}
	defer tx.Rollback(ctx)
	var before domain.FamilyEntry
	if e.ID != 0 {
		before, err = scanFamilyEntry(tx.QueryRow(ctx, "select "+familyColumns+" from family_entries where id=$1 for update", e.ID))
		if err != nil {
			return e, err
		}
		if before.Version != e.Version {
			return e, domain.ErrConflict
		}
	}
	data, err := json.Marshal(e)
	if err != nil {
		return e, err
	}
	var saved domain.FamilyEntry
	if e.ID == 0 {
		saved, err = scanFamilyEntry(tx.QueryRow(ctx, "insert into family_entries(author_id,data) values($1,$2) returning "+familyColumns, e.AuthorID, data))
	} else {
		saved, err = scanFamilyEntry(tx.QueryRow(ctx, "update family_entries set data=$1,version=version+1,updated_at=now() where id=$2 and version=$3 returning "+familyColumns, data, e.ID, e.Version))
	}
	if err != nil {
		return e, err
	}
	for _, reward := range domain.FamilyRewardCandidates(before, saved) {
		if err = insertActivityReward(ctx, tx, reward); err != nil {
			return e, err
		}
	}
	return saved, tx.Commit(ctx)
}
