package store

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lobov/familyquest/backend/internal/domain"
)

func LOVITSubject(family int64) string { return "familyquest:" + strconv.FormatInt(family, 10) }

// BindLOVIT is an explicit operator action, restricted to unbilled expired families.
// Existing lifetime grants and payment histories cannot be silently migrated.
func (p *Platform) BindLOVIT(ctx context.Context, family int64, subscription, operator string) error {
	if family <= 0 || !strings.HasPrefix(subscription, "sub_") || len(subscription) > 128 || strings.TrimSpace(operator) == "" {
		return domain.ErrInvalidInput
	}
	tx, err := p.root.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx)
	var schema, status string
	if err = tx.QueryRow(ctx, `select schema_name,status from fq_platform.families where id=$1 for update`, family).Scan(&schema, &status); err != nil {
		return err
	}
	var prior string
	err = tx.QueryRow(ctx, `select subscription_id from fq_platform.lovit_bindings where family_id=$1`, family).Scan(&prior)
	if err == nil {
		if prior != subscription {
			return domain.ErrConflict
		}
		return tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var until *time.Time
	var history bool
	if err = tx.QueryRow(ctx, "select access_until from "+pgx.Identifier{schema}.Sanitize()+".service_policy where id for update").Scan(&until); err != nil {
		return err
	}
	if err = tx.QueryRow(ctx, `select exists(select 1 from fq_platform.payments where family_id=$1)`, family).Scan(&history); err != nil {
		return err
	}
	if status != "active" || until == nil || until.After(time.Now()) || history {
		return domain.ErrConflict
	}
	if _, err = tx.Exec(ctx, `insert into fq_platform.lovit_bindings(family_id,subscription_id) values($1,$2)`, family, subscription); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "update "+pgx.Identifier{schema}.Sanitize()+".service_policy set billing_managed=true where id"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `insert into fq_platform.audit(family_id,action,operator) values($1,'bind-lovit',$2)`, family, operator); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p *Platform) LOVITBinding(ctx context.Context, family int64) (string, error) {
	var subscription string
	err := p.root.pool.QueryRow(ctx, `select subscription_id from fq_platform.lovit_bindings where family_id=$1`, family).Scan(&subscription)
	return subscription, err
}

// ApplyLOVIT projects rights into the product policy. No account, profile, PIN or
// payment is created or modified. Failure to fetch a new revision leaves the last
// purchased expiry unchanged; a network outage cannot extend access indefinitely.
func (p *Platform) ApplyLOVIT(ctx context.Context, family int64, e domain.LOVITEntitlement, operator string) error {
	if strings.TrimSpace(operator) == "" {
		return domain.ErrInvalidInput
	}
	tx, err := p.root.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx)
	var schema string
	if err = tx.QueryRow(ctx, `select schema_name from fq_platform.families where id=$1 for update`, family).Scan(&schema); err != nil {
		return err
	}
	var subscription string
	var revision int64
	if err = tx.QueryRow(ctx, `select subscription_id,revision from fq_platform.lovit_bindings where family_id=$1 for update`, family).Scan(&subscription, &revision); err != nil {
		return err
	}
	if err = e.Validate(LOVITSubject(family), subscription, time.Now()); err != nil {
		return err
	}
	if e.Revision < revision {
		return domain.ErrConflict
	}
	if e.Revision == revision {
		return tx.Commit(ctx)
	}
	// Lock order matches operator payments/refunds. Never overwrite an unexpected
	// local lifetime grant or an independently received local payment.
	var oldUntil *time.Time
	var managed, history bool
	if err = tx.QueryRow(ctx, "select access_until,billing_managed from "+pgx.Identifier{schema}.Sanitize()+".service_policy where id for update").Scan(&oldUntil, &managed); err != nil {
		return err
	}
	if err = tx.QueryRow(ctx, `select exists(select 1 from fq_platform.payments where family_id=$1)`, family).Scan(&history); err != nil {
		return err
	}
	if !managed || oldUntil == nil || history {
		return domain.ErrConflict
	}
	until := time.Now().UTC()
	active := e.Access == "active"
	if active {
		until = *e.ValidUntil
	}
	if _, err = tx.Exec(ctx, "update "+pgx.Identifier{schema}.Sanitize()+".service_policy set access_until=$1,children_unlimited=$2 where id", until, active); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `update fq_platform.lovit_bindings set revision=$2,active=$3,access_until=$4,updated_at=now() where family_id=$1`, family, e.Revision, active, until); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `insert into fq_platform.audit(family_id,action,operator) values($1,'sync-lovit',$2)`, family, operator); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Platform) HasPaymentHistory(ctx context.Context, family int64) (bool, error) {
	var yes bool
	err := p.root.pool.QueryRow(ctx, `select exists(select 1 from fq_platform.payments where family_id=$1)`, family).Scan(&yes)
	return yes, err
}
