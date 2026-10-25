package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/lobov/familyquest/backend/internal/domain"
	"github.com/lobov/familyquest/backend/internal/lovit"
)

func TestLOVITProjectionPreservesLegacyAndSeparatesPayments(t *testing.T) {
	raw := os.Getenv("FAMILYQUEST_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("dedicated disposable database required")
	}
	ctx := context.Background()
	root, err := Open(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	t.Chdir("../..")
	p := NewPlatform(root)
	if err = p.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprint(time.Now().UnixNano())
	ids := []int64{}
	defer func() {
		for _, id := range ids {
			_, _ = root.pool.Exec(ctx, `delete from fq_platform.audit where family_id=$1`, id)
			_, _ = root.pool.Exec(ctx, `delete from fq_platform.payments where family_id=$1`, id)
			_, _ = root.pool.Exec(ctx, `delete from fq_platform.accounts where family_id=$1`, id)
			_, _ = root.pool.Exec(ctx, `delete from fq_platform.families where id=$1`, id)
			_, _ = root.pool.Exec(ctx, "drop schema "+schemaFor(id)+" cascade")
			_, _ = root.pool.Exec(ctx, "drop role "+roleFor(id))
		}
	}()
	for i := 0; i < 3; i++ {
		id, e := p.Provision(ctx, fmt.Sprint("Family", i), fmt.Sprint(i, suffix, "@example.test"), "long-password-test", "Parent", "739281", "test", false)
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
	}
	// Existing permanent grant must remain unchanged and cannot be opted into paid billing.
	if _, err = root.pool.Exec(ctx, "update "+schemaFor(ids[1])+".service_policy set access_until=null"); err != nil {
		t.Fatal(err)
	}
	if err = p.BindLOVIT(ctx, ids[1], "sub_permanent", "test"); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("permanent migrated", err)
	}
	legacy, err := p.Subscription(ctx, ids[1])
	if err != nil || legacy.AccessUntil != nil || legacy.BillingManaged || !legacy.Access {
		t.Fatal("legacy changed", err)
	}
	payment := ManualPayment{FamilyID: ids[2], Reference: "history-" + suffix, AmountMinor: 10000, Currency: "RUB", StartsAt: time.Now().Add(-time.Minute), EndsAt: time.Now().Add(time.Hour), ChildLimit: 3, Operator: "test"}
	if err = p.RecordPayment(ctx, payment); err != nil {
		t.Fatal(err)
	}
	if err = p.BindLOVIT(ctx, ids[2], "sub_history", "test"); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("existing payment migrated", err)
	}
	family := ids[0]
	if err = p.BindLOVIT(ctx, family, "sub_paid", "test"); err != nil {
		t.Fatal(err)
	}
	if err = p.BindLOVIT(ctx, family, "sub_paid", "test"); err != nil {
		t.Fatal("binding retry", err)
	}
	if err = p.BindLOVIT(ctx, family, "sub_other", "test"); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("binding reassigned", err)
	}
	until := time.Now().Add(2 * time.Hour).Truncate(time.Microsecond)
	rights := lovit.Entitlement{ProductID: "familyquest", SubscriptionID: "sub_paid", SubjectRef: LOVITSubject(family), Access: "active", Paid: true, ValidUntil: &until, Revision: 1, Features: map[string]bool{"children_unlimited": true}, EvaluatedAt: time.Now()}
	if err = p.ApplyLOVIT(ctx, family, rights, "test"); err != nil {
		t.Fatal(err)
	}
	sub, err := p.Subscription(ctx, family)
	if err != nil || !sub.Paid || !sub.ChildrenUnlimited || !sub.BillingManaged || !sub.Access {
		t.Fatal("projection", sub, err)
	}
	f, err := p.Family(ctx, family)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if _, err = f.CreateParticipant(ctx, Participant{Name: fmt.Sprint("Child", i), Role: domain.RoleChild}, "739281"); err != nil {
			t.Fatal("unlimited child count", err)
		}
	}
	wrong := rights
	wrong.SubjectRef = LOVITSubject(ids[1])
	if err = p.ApplyLOVIT(ctx, family, wrong, "test"); err == nil {
		t.Fatal("cross family")
	}
	stale := rights
	stale.Revision = 0
	if err = p.ApplyLOVIT(ctx, family, stale, "test"); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("stale replay", err)
	}
	longer := until.Add(24 * time.Hour)
	same := rights
	same.ValidUntil = &longer
	if err = p.ApplyLOVIT(ctx, family, same, "test"); err != nil {
		t.Fatal(err)
	}
	sub, err = p.Subscription(ctx, family)
	if err != nil || !sub.AccessUntil.Equal(until) {
		t.Fatal("same revision extended access", err)
	}
	var count int
	if err = root.pool.QueryRow(ctx, `select count(*) from fq_platform.payments where family_id=$1`, family).Scan(&count); err != nil || count != 0 {
		t.Fatal("payment duplicated", count, err)
	}
	payment.FamilyID = family
	payment.Reference = "dual-" + suffix
	if err = p.RecordPayment(ctx, payment); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("dual billing accepted", err)
	}
	rights.Revision = 2
	rights.Access = "expired"
	rights.Paid = false
	rights.ValidUntil = nil
	if err = p.ApplyLOVIT(ctx, family, rights, "test"); err != nil {
		t.Fatal(err)
	}
	sub, err = p.Subscription(ctx, family)
	if err != nil || sub.Access || sub.Paid || sub.ActiveChildren != 4 {
		t.Fatal("revocation lost profiles or failed", sub, err)
	}
	if _, err = f.CreateParticipant(ctx, Participant{Name: "No access", Role: domain.RoleChild}, "739281"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal("expired writes allowed", err)
	}
	legacy, err = p.Subscription(ctx, ids[1])
	if err != nil || legacy.AccessUntil != nil || !legacy.Access {
		t.Fatal("other family affected", err)
	}
}
