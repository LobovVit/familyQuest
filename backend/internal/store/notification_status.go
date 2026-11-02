package store

import (
	"context"
	"encoding/json"
	"time"
)

// NotificationStatus is an operator-only aggregate, with no recipient identity or payload.
type NotificationStatus struct {
	FamilyID          int64            `json:"familyId"`
	EnabledRecipients int64            `json:"enabledRecipients"`
	Pending           int64            `json:"pending"`
	Leased            int64            `json:"leased"`
	Retrying          int64            `json:"retrying"`
	Delivered         int64            `json:"deliveredToPortal"`
	Cancelled         int64            `json:"cancelled"`
	Failed            int64            `json:"failed"`
	MaxAttempts       int              `json:"maxAttempts"`
	OldestPending     *time.Time       `json:"oldestPending"`
	Errors            map[string]int64 `json:"unresolvedErrors"`
}

func (s *Store) NotificationStatus(ctx context.Context) (NotificationStatus, error) {
	v := NotificationStatus{FamilyID: s.familyID}
	var raw []byte
	e := s.db.QueryRow(ctx, `select
 (select count(*) from notification_recipients where enabled),
 count(*) filter(where state='pending'),
 count(*) filter(where state='pending' and lease_until>now()),
 count(*) filter(where state='pending' and attempts>0),
 count(*) filter(where state='delivered'),count(*) filter(where state='cancelled'),
 count(*) filter(where state='failed'),coalesce(max(attempts),0),
 min(created_at) filter(where state='pending'),
 (select coalesce(jsonb_object_agg(error_code,n),'{}'::jsonb) from
   (select error_code,count(*) n from notification_outbox where state in ('pending','failed') and error_code<>'' group by error_code) e)
 from notification_outbox`).Scan(&v.EnabledRecipients, &v.Pending, &v.Leased, &v.Retrying, &v.Delivered, &v.Cancelled, &v.Failed, &v.MaxAttempts, &v.OldestPending, &raw)
	if e != nil {
		return v, e
	}
	e = json.Unmarshal(raw, &v.Errors)
	return v, e
}
