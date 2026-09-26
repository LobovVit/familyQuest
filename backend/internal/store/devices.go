package store

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/lobov/familyquest/backend/internal/domain"
	"time"
)

func (s *Store) CreateDevice(ctx context.Context, d domain.TrustedDevice, hash string, owner, approver domain.Participant, previousHash string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `select id,role,session_version from participants where id=any($1) and active order by id for share`, []int64{owner.ID, approver.ID})
	if err != nil {
		return err
	}
	validOwner, validApprover := false, owner.Role == domain.RoleParent
	for rows.Next() {
		var id, version int64
		var role string
		if err = rows.Scan(&id, &role, &version); err != nil {
			rows.Close()
			return err
		}
		if id == owner.ID && role == owner.Role && version == owner.SessionVersion {
			validOwner = true
		}
		if id == approver.ID && role == domain.RoleParent && version == approver.SessionVersion {
			validApprover = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if !validOwner || !validApprover {
		return domain.ErrUnauthorized
	}
	_, err = tx.Exec(ctx, `insert into trusted_devices(id,participant_id,session_version,token_hash,name,created_at,last_seen_at,expires_at) values($1,$2,$3,$4,$5,$6,$7,$8)`, d.ID, owner.ID, owner.SessionVersion, hash, d.Name, d.CreatedAt, d.LastSeenAt, d.ExpiresAt)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `update trusted_devices set revoked_at=coalesce(revoked_at,now()) where token_hash=$1`, previousHash); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) AuthenticateDevice(ctx context.Context, hash string, now, expires time.Time) (domain.Participant, domain.TrustedDevice, error) {
	var p domain.Participant
	var d domain.TrustedDevice
	err := s.db.QueryRow(ctx, `update trusted_devices d set last_seen_at=$2,expires_at=$3 from participants p where d.token_hash=$1 and d.revoked_at is null and d.expires_at>$2 and p.id=d.participant_id and p.active and p.role in ('parent','child') and p.session_version=d.session_version returning p.id,p.name,p.role,p.active,p.created_at,p.session_version,d.id,d.name,d.created_at,d.last_seen_at,d.expires_at`, hash, now, expires).Scan(&p.ID, &p.Name, &p.Role, &p.Active, &p.CreatedAt, &p.SessionVersion, &d.ID, &d.Name, &d.CreatedAt, &d.LastSeenAt, &d.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = domain.ErrUnauthorized
	}
	p.FamilyID = s.familyID
	d.ParticipantID = p.ID
	d.ParticipantName = p.Name
	d.Current = true
	return p, d, err
}
func (s *Store) ListDevices(ctx context.Context) ([]domain.TrustedDevice, error) {
	rows, err := s.db.Query(ctx, `select d.id,p.id,p.name,d.name,d.created_at,d.last_seen_at,d.expires_at from trusted_devices d join participants p on p.id=d.participant_id where d.revoked_at is null and d.expires_at>now() and p.active and p.session_version=d.session_version order by d.last_seen_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.TrustedDevice{}
	for rows.Next() {
		var d domain.TrustedDevice
		if err = rows.Scan(&d.ID, &d.ParticipantID, &d.ParticipantName, &d.Name, &d.CreatedAt, &d.LastSeenAt, &d.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) RevokeDevice(ctx context.Context, id string) error {
	tag, err := s.db.Exec(ctx, `update trusted_devices set revoked_at=coalesce(revoked_at,now()) where id=$1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return err
}
func (s *Store) RevokeDeviceToken(ctx context.Context, hash string) error {
	_, err := s.db.Exec(ctx, `update trusted_devices set revoked_at=coalesce(revoked_at,now()) where token_hash=$1`, hash)
	return err
}

func (s *Store) DeviceAuthorized(ctx context.Context, id string, owner, version int64) error {
	var active bool
	err := s.db.QueryRow(ctx, `select exists(select 1 from trusted_devices where id=$1 and participant_id=$2 and session_version=$3 and revoked_at is null and expires_at>now())`, id, owner, version).Scan(&active)
	if err != nil {
		return err
	}
	if !active {
		return domain.ErrUnauthorized
	}
	return nil
}
