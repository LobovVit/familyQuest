package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/lobov/familyquest/backend/internal/domain"
)

func (s *Store) SetNotificationRecipient(ctx context.Context, v domain.NotificationRecipient, enabled bool) error {
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if enabled {
		// Serializes with parent changes; the application check alone is insufficient.
		var allowed bool
		e = tx.QueryRow(ctx, `select active and role='parent' from participants where id=$1 for share`, v.ParentID).Scan(&allowed)
		if e != nil {
			return e
		}
		if !allowed {
			return domain.ErrForbidden
		}
		_, e = tx.Exec(ctx, `insert into notification_recipients(recipient_ref,subscription_id,issuer,subject,parent_id) values($1,$2,$3,$4,$5)
    on conflict(recipient_ref) do update set subscription_id=excluded.subscription_id,issuer=excluded.issuer,subject=excluded.subject,parent_id=excluded.parent_id,enabled=true,updated_at=now()`, v.Reference, v.SubscriptionID, v.Issuer, v.Subject, v.ParentID)
	} else {
		_, e = tx.Exec(ctx, `update notification_recipients set enabled=false,updated_at=now() where recipient_ref=$1 and subscription_id=$2`, v.Reference, v.SubscriptionID)
	}
	if e != nil {
		return e
	}
	// A fresh consent generation cannot resurrect old pending events.
	_, e = tx.Exec(ctx, `update notification_outbox o set state='cancelled',error_code='recipient_revoked',lease_token=null,lease_until=null,settled_at=now()
 where state='pending' and recipient_ref=$1 and not exists(select 1 from notification_recipients r where r.recipient_ref=o.recipient_ref and r.subscription_id=o.subscription_id and r.enabled)`, v.Reference)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (s *Store) ClaimNotification(ctx context.Context) (*domain.NotificationDelivery, error) {
	var v domain.NotificationDelivery
	e := s.db.QueryRow(ctx, `with candidate as (
 select id from notification_outbox where state='pending' and available_at<=now() and (lease_until is null or lease_until<=now())
 order by available_at,created_at,id for update skip locked limit 1)
 update notification_outbox o set lease_token=gen_random_uuid(),lease_until=now()+interval '60 seconds',attempts=least(attempts+1,2147483646)
 from candidate c where o.id=c.id returning o.id::text,o.lease_token::text,o.recipient_ref,o.subscription_id::text,o.issuer,o.subject,o.parent_id,o.attempts`).Scan(&v.ID, &v.Lease, &v.Recipient.Reference, &v.Recipient.SubscriptionID, &v.Recipient.Issuer, &v.Recipient.Subject, &v.Recipient.ParentID, &v.Attempts)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return &v, nil
}
func (s *Store) NotificationAllowed(ctx context.Context, id, lease string) (bool, error) {
	var allowed bool
	e := s.db.QueryRow(ctx, `select exists(select 1 from notification_outbox o join notification_recipients r on r.recipient_ref=o.recipient_ref and r.subscription_id=o.subscription_id
 join participants p on p.id=o.parent_id where o.id=$1 and o.lease_token=$2 and o.state='pending' and o.lease_until>now() and r.enabled and p.active and p.role='parent'
 and r.parent_id=o.parent_id and r.issuer=o.issuer and r.subject=o.subject)`, id, lease).Scan(&allowed)
	return allowed, e
}
func (s *Store) SettleNotification(ctx context.Context, v domain.NotificationSettlement) error {
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var attempts int
	e = tx.QueryRow(ctx, `select attempts from notification_outbox where id=$1 and lease_token=$2 and state='pending' and lease_until>now() for update`, v.ID, v.Lease).Scan(&attempts)
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.ErrConflict
	}
	if e != nil {
		return e
	}
	state := v.Outcome
	delay := 0.0
	if state == "retry" {
		state = "pending"
		delay = domain.NotificationRetryDelay(attempts).Seconds()
	}
	_, e = tx.Exec(ctx, `update notification_outbox set state=$2,error_code=$3,portal_id=$4,available_at=now()+make_interval(secs=>$5),lease_token=null,lease_until=null,settled_at=case when $2='pending' then null else now() end where id=$1`, v.ID, state, v.ErrorCode, v.PortalID, delay)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// Access can discover families using its existing read-only catalog grant; it never reads family tables.
func (p *Platform) NotificationFamilies(ctx context.Context, after int64) ([]int64, error) {
	rows, e := p.root.pool.Query(ctx, `select id from fq_platform.families where id>$1 and status='active' order by id limit 50`, after)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			return nil, e
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
