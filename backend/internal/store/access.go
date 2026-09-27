package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lobov/familyquest/backend/internal/domain"
)

// CorePlatform exposes only routing and local entitlement projection, never billing credentials.
// CorePlatform читает только маршрутизацию и локальную проекцию доступа, без паролей аккаунтов и оплат.
type CorePlatform struct{ *Platform }

func (p CorePlatform) Account(context.Context, string, string) (int64, int64, error) {
	return 0, 0, domain.ErrForbidden
}
func (p CorePlatform) Subscription(ctx context.Context, id int64) (domain.FamilySubscription, error) {
	v := domain.FamilySubscription{FamilyID: id}
	var schema string
	e := p.root.pool.QueryRow(ctx, `select name,status,created_at,plan,schema_name from fq_platform.families where id=$1`, id).Scan(&v.Name, &v.Status, &v.RegisteredAt, &v.Plan, &schema)
	if e != nil {
		return v, e
	}
	db := &familyDB{pool: p.root.pool, schema: schema, role: roleFor(id)}
	e = db.QueryRow(ctx, `select child_limit,access_until,(select count(*)::int from participants where active and role='child') from service_policy where id`).Scan(&v.ChildLimit, &v.AccessUntil, &v.ActiveChildren)
	v.Access = v.Status == "active" && (v.AccessUntil == nil || time.Now().Before(*v.AccessUntil))
	return v, e
}
func (p *Platform) Paid(ctx context.Context, id int64) (bool, error) {
	var paid bool
	e := p.root.pool.QueryRow(ctx, `select exists(select 1 from fq_platform.payments where family_id=$1 and starts_at<=now() and ends_at>now() and refunded_at is null)`, id).Scan(&paid)
	return paid, e
}

// ConfigureAccess grants only catalog reads; family roles must not be inherited or assumable.
// ConfigureAccess выдаёт только чтение каталога, запрещая членство в семейных ролях.
func (p *Platform) ConfigureAccess(ctx context.Context, role string) error {
	tx, e := p.root.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer rollback(tx)
	var safe bool
	e = tx.QueryRow(ctx, `select not rolsuper and not rolcreaterole and not rolcreatedb and not rolbypassrls and not exists(select 1 from pg_auth_members where member=pg_roles.oid) from pg_roles where rolname=$1`, role).Scan(&safe)
	if e != nil {
		return e
	}
	if !safe {
		return fmt.Errorf("access role requires no administrative attributes or memberships")
	}
	name := pgx.Identifier{role}.Sanitize()
	if _, e = tx.Exec(ctx, "alter role "+name+" noinherit; grant usage on schema fq_platform to "+name+"; grant select on fq_platform.accounts,fq_platform.families,fq_platform.payments to "+name); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (p *Platform) ConfigureCore(ctx context.Context, role string) error {
	return p.configureRuntime(ctx, role, false)
}
func (p *Platform) CheckCoreRuntime(ctx context.Context) error {
	if e := p.CheckRuntime(ctx); e != nil {
		return e
	}
	var leaks bool
	e := p.root.pool.QueryRow(ctx, `select has_table_privilege(current_user,'fq_platform.accounts','select') or has_table_privilege(current_user,'fq_platform.payments','select')`).Scan(&leaks)
	if e != nil {
		return e
	}
	if leaks {
		return fmt.Errorf("core runtime must not read accounts or payments")
	}
	return nil
}
func (p *Platform) CheckAccessRuntime(ctx context.Context) error {
	if e := p.CheckRuntime(ctx); e != nil {
		return e
	}
	var members int
	if e := p.root.pool.QueryRow(ctx, `select count(*) from pg_auth_members m join pg_roles r on r.oid=m.member where r.rolname=current_user`).Scan(&members); e != nil {
		return e
	}
	var leaks bool
	if e := p.root.pool.QueryRow(ctx, `select exists(select 1 from pg_class c join pg_namespace n on n.oid=c.relnamespace where c.relkind in ('r','p','v','m','f') and (n.nspname='public' or n.nspname like 'fq_family_%') and has_table_privilege(current_user,c.oid,'SELECT,INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER'))`).Scan(&leaks); e != nil {
		return e
	}
	if leaks {
		return fmt.Errorf("access runtime must not access family tables")
	}
	var catalogSafe bool
	if e := p.root.pool.QueryRow(ctx, `select bool_and(has_table_privilege(current_user,c.oid,'SELECT') and not has_table_privilege(current_user,c.oid,'INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER')) from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='fq_platform' and c.relname in ('accounts','families','payments')`).Scan(&catalogSafe); e != nil {
		return e
	}
	if !catalogSafe {
		return fmt.Errorf("access runtime requires read-only catalog privileges")
	}
	if members != 0 {
		return fmt.Errorf("access runtime must not have role memberships")
	}
	return nil
}
