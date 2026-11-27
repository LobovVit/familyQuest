package store

import (
	"context"

	"github.com/lobov/familyquest/backend/internal/application"
	"github.com/lobov/familyquest/backend/internal/domain"
)

func (s *Store) CreateHelperAccess(ctx context.Context, hash []byte, label string, taskIDs []int64) (application.HelperAccess, error) {
	access := application.HelperAccess{}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return access, err
	}
	defer rollback(tx)

	err = tx.QueryRow(ctx, `insert into helper_access(secret_hash,label) values ($1,$2)
		returning id,label,expires_at`, hash, label).Scan(&access.ID, &access.Label, &access.ExpiresAt)
	if err != nil {
		return access, err
	}
	tag, err := tx.Exec(ctx, `insert into helper_tasks(access_id,task_id,title)
		select $1,t.id,c.title from tasks t
		join assignments a on a.id=t.assignment_id and a.active
		join participants p on p.id=a.participant_id and p.active
		join chores c on c.id=a.chore_id and c.active
		where t.id=any($2::bigint[])`, access.ID, taskIDs)
	if err != nil {
		return access, err
	}
	if tag.RowsAffected() != int64(len(taskIDs)) {
		return access, domain.ErrInvalidInput
	}
	return access, tx.Commit(ctx)
}

func (s *Store) ListHelperAccess(ctx context.Context) ([]application.HelperAccess, error) {
	rows, err := s.db.Query(ctx, `select id,label,expires_at from helper_access where expires_at > now() order by id desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []application.HelperAccess{}
	for rows.Next() {
		var access application.HelperAccess
		if err := rows.Scan(&access.ID, &access.Label, &access.ExpiresAt); err != nil {
			return nil, err
		}
		result = append(result, access)
	}
	return result, rows.Err()
}

func (s *Store) RevokeHelperAccess(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `delete from helper_access where id=$1`, id)
	return err
}

func (s *Store) HelperTasks(ctx context.Context, hash []byte) ([]application.HelperTask, error) {
	rows, err := s.db.Query(ctx, `select item.title,to_char(item.due_date,'YYYY-MM-DD') from helper_access h
		left join lateral (
			select ht.title, t.due_date from helper_tasks ht
			join tasks t on t.id=ht.task_id
			join assignments a on a.id=t.assignment_id and a.active
			join participants p on p.id=a.participant_id and p.active
			join chores c on c.id=a.chore_id and c.active
			where ht.access_id=h.id
		) item on true
		where h.secret_hash=$1 and h.expires_at > now()
		order by item.due_date,item.title`, hash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []application.HelperTask{}
	valid := false
	for rows.Next() {
		valid = true
		var title, date *string
		if err := rows.Scan(&title, &date); err != nil {
			return nil, err
		}
		if title != nil && date != nil {
			result = append(result, application.HelperTask{Title: *title, Date: *date})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if !valid {
		return nil, domain.ErrUnauthorized
	}
	return result, nil
}
