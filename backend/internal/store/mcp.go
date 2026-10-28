package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lobov/familyquest/backend/internal/domain"
)

func (s *Store) CreateMCPGrant(ctx context.Context, g domain.MCPGrant, version int64) error {
	tag, err := s.db.Exec(ctx, `insert into mcp_grants(id,participant_id,session_version,name,read_only,created_at,expires_at) select $1,id,$3,$4,$5,$6,$7 from participants where id=$2 and active and session_version=$3`, g.ID, g.ParticipantID, version, g.Name, g.ReadOnly, g.CreatedAt, g.ExpiresAt)
	if err == nil && tag.RowsAffected() != 1 {
		return domain.ErrUnauthorized
	}
	return err
}
func (s *Store) MCPGrant(ctx context.Context, id string, owner, version int64, now time.Time) (domain.MCPGrant, error) {
	var g domain.MCPGrant
	err := s.db.QueryRow(ctx, `select id,participant_id,name,read_only,created_at,expires_at from mcp_grants where id=$1 and participant_id=$2 and session_version=$3 and revoked_at is null and expires_at>$4`, id, owner, version, now).Scan(&g.ID, &g.ParticipantID, &g.Name, &g.ReadOnly, &g.CreatedAt, &g.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = domain.ErrUnauthorized
	}
	return g, err
}
func (s *Store) ListMCPGrants(ctx context.Context) ([]domain.MCPGrant, error) {
	rows, err := s.db.Query(ctx, `select g.id,g.participant_id,g.name,g.read_only,g.created_at,g.expires_at from mcp_grants g join participants p on p.id=g.participant_id where g.revoked_at is null and g.expires_at>now() and p.active and p.session_version=g.session_version order by g.created_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.MCPGrant{}
	for rows.Next() {
		var g domain.MCPGrant
		if err = rows.Scan(&g.ID, &g.ParticipantID, &g.Name, &g.ReadOnly, &g.CreatedAt, &g.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
func (s *Store) RevokeMCPGrant(ctx context.Context, id string) error {
	tag, err := s.db.Exec(ctx, `update mcp_grants set revoked_at=coalesce(revoked_at,now()) where id=$1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return err
}
