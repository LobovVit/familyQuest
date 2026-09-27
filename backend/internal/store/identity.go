package store

import (
	"context"
	"errors"
	"net/url"

	"github.com/jackc/pgx/v5"
	"github.com/lobov/familyquest/backend/internal/domain"
)

// BindIdentity is an explicit operator binding; matching email never links accounts.
// BindIdentity — явная привязка оператором; совпадение email не связывает аккаунты.
func (p *Platform) BindIdentity(ctx context.Context, issuer, subject string, family int64, operator string) error {
	u, e := url.Parse(issuer)
	if e != nil || u.Host == "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))) || u.User != nil || u.RawQuery != "" || u.Fragment != "" || subject == "" || len(subject) > 512 || family <= 0 || operator == "" {
		return domain.ErrInvalidInput
	}
	tx, e := p.root.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer rollback(tx)
	var exists int64
	e = tx.QueryRow(ctx, `select family_id from fq_platform.identities where issuer=$1 and subject=$2`, issuer, subject).Scan(&exists)
	if e == nil {
		if exists == family {
			return nil
		}
		return domain.ErrConflict
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	if _, e = tx.Exec(ctx, `insert into fq_platform.identities(issuer,subject,family_id) values($1,$2,$3)`, issuer, subject, family); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `insert into fq_platform.audit(family_id,action,operator) values($1,'bind-identity',$2)`, family, operator); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (p *Platform) Identity(ctx context.Context, issuer, subject string) (int64, int64, error) {
	var family, owner int64
	e := p.root.pool.QueryRow(ctx, `select i.family_id,a.participant_id from fq_platform.identities i join fq_platform.accounts a on a.family_id=i.family_id join fq_platform.families f on f.id=i.family_id where i.issuer=$1 and i.subject=$2 and f.status='active'`, issuer, subject).Scan(&family, &owner)
	if errors.Is(e, pgx.ErrNoRows) {
		e = domain.ErrUnauthorized
	}
	return family, owner, e
}
