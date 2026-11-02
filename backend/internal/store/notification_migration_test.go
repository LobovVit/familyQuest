package store

import (
	"context"
	"os"
	"testing"
)

func TestNotificationBindingUpgradeRevokesLegacyWithoutLosingAudit(t *testing.T) {
	url := os.Getenv("FAMILYQUEST_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("disposable postgres required")
	}
	t.Chdir("../..")
	ctx := context.Background()
	s, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `create schema binding_upgrade_test; set local search_path=binding_upgrade_test`); err != nil {
		t.Fatal(err)
	}
	legacy, err := os.ReadFile("migrations/011_notification_outbox.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, string(legacy)); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `insert into notification_recipients values('tgr_legacy',gen_random_uuid(),'issuer','subject',1,true,now());
 insert into notification_outbox(task_id,recipient_ref,subscription_id,issuer,subject,parent_id,state,lease_token,lease_until)
 select 1,recipient_ref,subscription_id,issuer,subject,parent_id,'pending',gen_random_uuid(),now()+interval '1 minute' from notification_recipients;
 insert into notification_outbox(task_id,recipient_ref,subscription_id,issuer,subject,parent_id,state,portal_id)
 select 2,recipient_ref,subscription_id,issuer,subject,parent_id,'delivered','tgn_audit' from notification_recipients;`); err != nil {
		t.Fatal(err)
	}
	upgrade, err := os.ReadFile("migrations/012_notification_bindings.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, string(upgrade)); err != nil {
		t.Fatal(err)
	}
	var enabled, cancelled, delivered int
	if err = tx.QueryRow(ctx, `select
 (select count(*) from notification_recipients where enabled),
 (select count(*) from notification_outbox where state='cancelled' and error_code='binding_upgrade' and lease_token is null and lease_until is null and settled_at is not null),
 (select count(*) from notification_outbox where state='delivered' and portal_id='tgn_audit')`).Scan(&enabled, &cancelled, &delivered); err != nil || enabled != 0 || cancelled != 1 || delivered != 1 {
		t.Fatal("unsafe legacy upgrade", enabled, cancelled, delivered, err)
	}
}
