package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lobov/familyquest/backend/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

// Platform is used only by the operator and account gateway, never family SQL.
// Platform используется оператором и входом аккаунта, не семейными SQL-сценариями.
type Platform struct{ root *Store }

func NewPlatform(root *Store) *Platform { return &Platform{root} }
func (p *Platform) Migrate(ctx context.Context) error {
	_, e := p.root.pool.Exec(ctx, `create schema if not exists fq_platform;
 create table if not exists fq_platform.families(id bigserial primary key,name text not null,schema_name text not null unique,status text not null default 'active' check(status in ('active','suspended')),created_at timestamptz not null default now(),plan text not null default 'registered');
 create table if not exists fq_platform.accounts(email text primary key,password_hash text not null,family_id bigint not null unique references fq_platform.families(id),participant_id bigint not null);
 create table if not exists fq_platform.payments(reference text primary key,family_id bigint not null references fq_platform.families(id),amount_minor bigint not null check(amount_minor>0),currency text not null,starts_at timestamptz not null,ends_at timestamptz not null,child_limit int not null check(child_limit>=0),created_at timestamptz not null default now(),operator text not null,check(ends_at>starts_at));
 create table if not exists fq_platform.audit(id bigserial primary key,family_id bigint not null,action text not null,operator text not null,created_at timestamptz not null default now());
 alter table fq_platform.payments add column if not exists refunded_at timestamptz;
 revoke all on schema fq_platform from public;revoke all on all tables in schema fq_platform from public;`)
	return e
}
func migrationFiles() ([]string, error) {
	dir := "migrations"
	if _, e := os.Stat(dir); e != nil {
		dir = "backend/migrations"
	}
	files, e := filepath.Glob(filepath.Join(dir, "*.sql"))
	sort.Strings(files)
	return files, e
}
func applyFamilyMigrations(ctx context.Context, tx pgx.Tx) error {
	if _, e := tx.Exec(ctx, `create table if not exists schema_migrations(version text primary key,applied_at timestamptz not null default now())`); e != nil {
		return e
	}
	files, e := migrationFiles()
	if e != nil || len(files) == 0 {
		return fmt.Errorf("migration files unavailable: %v", e)
	}
	for _, file := range files {
		var done bool
		if e = tx.QueryRow(ctx, `select exists(select 1 from schema_migrations where version=$1)`, filepath.Base(file)).Scan(&done); e != nil {
			return e
		}
		if done {
			continue
		}
		b, e := os.ReadFile(file)
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, string(b)); e != nil {
			return fmt.Errorf("%s: %w", file, e)
		}
		if _, e = tx.Exec(ctx, `insert into schema_migrations(version) values($1)`, filepath.Base(file)); e != nil {
			return e
		}
	}
	return nil
}
func emailKey(v string) (string, error) {
	v = strings.ToLower(strings.TrimSpace(v))
	a, e := mail.ParseAddress(v)
	if e != nil || a.Address != v || len(v) > 254 {
		return "", domain.ErrInvalidInput
	}
	return v, nil
}
func (p *Platform) Provision(ctx context.Context, name, email, password, parent, pin, operator string, legacy bool) (int64, error) {
	email, e := emailKey(email)
	if e != nil || len(password) < 12 || len(password) > 72 || strings.TrimSpace(name) == "" || strings.TrimSpace(parent) == "" || domain.ValidatePIN(pin) != nil || operator == "" {
		return 0, domain.ErrInvalidInput
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if e != nil {
		return 0, e
	}
	ph, e := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if e != nil {
		return 0, e
	}
	tx, e := p.root.pool.Begin(ctx)
	if e != nil {
		return 0, e
	}
	defer rollback(tx)
	var id int64
	// Serialize catalog DDL and reject duplicate email before creating a role.
	// Сериализуем DDL каталога и проверяем адрес до создания роли.
	if _, e = tx.Exec(ctx, `select pg_advisory_xact_lock(88340219)`); e != nil {
		return 0, e
	}
	var exists bool
	if e = tx.QueryRow(ctx, `select exists(select 1 from fq_platform.accounts where email=$1)`, email).Scan(&exists); e != nil {
		return 0, e
	}
	if exists {
		return 0, domain.ErrConflict
	}
	raw := make([]byte, 12)
	if _, e = rand.Read(raw); e != nil {
		return 0, e
	}
	if e = tx.QueryRow(ctx, `insert into fq_platform.families(name,schema_name) values($1,$2) returning id`, name, "pending_"+hex.EncodeToString(raw)).Scan(&id); e != nil {
		return 0, e
	}
	schema := schemaFor(id)
	if legacy {
		schema = "public"
	}
	if _, e = tx.Exec(ctx, `update fq_platform.families set schema_name=$2 where id=$1`, id, schema); e != nil {
		return 0, e
	}
	if _, e = tx.Exec(ctx, "create schema if not exists "+pgx.Identifier{schema}.Sanitize()); e != nil {
		return 0, e
	}
	if _, e = tx.Exec(ctx, "select set_config('search_path',$1,true)", pgx.Identifier{schema}.Sanitize()+",pg_catalog"); e != nil {
		return 0, e
	}
	if e = applyFamilyMigrations(ctx, tx); e != nil {
		return 0, e
	}
	var owner int64
	if legacy {
		// Claim requires an existing parent's PIN and never changes their profile.
		// Присоединение требует PIN существующего родителя и не меняет профиль.
		var stored string
		e = tx.QueryRow(ctx, `select id,pin_hash from participants where name=$1 and role='parent' and active`, parent).Scan(&owner, &stored)
		if e != nil || bcrypt.CompareHashAndPassword([]byte(stored), []byte(pin)) != nil {
			return 0, domain.ErrUnauthorized
		}
	} else {
		e = tx.QueryRow(ctx, `insert into participants(name,role,pin_hash,pin_code) values($1,'parent',$2,null) returning id`, parent, string(ph)).Scan(&owner)
		if e != nil {
			return 0, e
		}
	}
	until := time.Now().UTC()
	var expiry *time.Time = &until
	if legacy {
		expiry = nil
	}
	childLimit := 3
	if legacy {
		if e = tx.QueryRow(ctx, `select greatest(3,count(*)::int) from participants where active and role='child'`).Scan(&childLimit); e != nil {
			return 0, e
		}
	}
	if _, e = tx.Exec(ctx, `update service_policy set child_limit=$1,access_until=$2,owner_participant_id=$3`, childLimit, expiry, owner); e != nil {
		return 0, e
	}
	role := pgx.Identifier{roleFor(id)}.Sanitize()
	if _, e = tx.Exec(ctx, "create role "+role+" nologin noinherit nosuperuser nocreatedb nocreaterole nobypassrls"); e != nil {
		return 0, e
	}
	var login string
	if e = tx.QueryRow(ctx, `select session_user`).Scan(&login); e != nil {
		return 0, e
	}
	if _, e = tx.Exec(ctx, "grant "+role+" to "+pgx.Identifier{login}.Sanitize()); e != nil {
		return 0, e
	}
	if e = grantFamily(ctx, tx, schema, role); e != nil {
		return 0, e
	}
	if _, e = tx.Exec(ctx, `insert into fq_platform.accounts(email,password_hash,family_id,participant_id) values($1,$2,$3,$4)`, email, string(hash), id, owner); e != nil {
		return 0, e
	}
	if _, e = tx.Exec(ctx, `insert into fq_platform.audit(family_id,action,operator) values($1,'provision',$2)`, id, operator); e != nil {
		return 0, e
	}
	return id, tx.Commit(ctx)
}
func grantFamily(ctx context.Context, tx pgx.Tx, schema, role string) error {
	sch := pgx.Identifier{schema}.Sanitize()
	_, e := tx.Exec(ctx, "grant usage on schema "+sch+" to "+role+"; grant select,insert,update,delete,truncate on all tables in schema "+sch+" to "+role+"; grant usage,select,update on all sequences in schema "+sch+" to "+role+"; revoke all on "+sch+".service_policy,"+sch+".schema_migrations from "+role+"; grant select on "+sch+".service_policy to "+role+"; grant update(id) on "+sch+".service_policy to "+role)
	return e
}
func (p *Platform) MigrateFamilies(ctx context.Context) error {
	rows, e := p.root.pool.Query(ctx, `select id,schema_name from fq_platform.families order by id`)
	if e != nil {
		return e
	}
	type f struct {
		id     int64
		schema string
	}
	fs := []f{}
	for rows.Next() {
		var v f
		if e = rows.Scan(&v.id, &v.schema); e != nil {
			rows.Close()
			return e
		}
		fs = append(fs, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, v := range fs {
		tx, e := p.root.pool.Begin(ctx)
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `select pg_advisory_xact_lock(88340219)`); e == nil {
			_, e = tx.Exec(ctx, `select set_config('search_path',$1,true)`, pgx.Identifier{v.schema}.Sanitize()+",pg_catalog")
		}
		if e == nil {
			e = applyFamilyMigrations(ctx, tx)
		}
		if e == nil {
			e = grantFamily(ctx, tx, v.schema, pgx.Identifier{roleFor(v.id)}.Sanitize())
		}
		if e == nil {
			e = tx.Commit(ctx)
		}
		rollback(tx)
		if e != nil {
			return e
		}
	}
	return nil
}
func (p *Platform) Family(ctx context.Context, id int64) (*Store, error) {
	var schema, status string
	e := p.root.pool.QueryRow(ctx, `select schema_name,status from fq_platform.families where id=$1`, id).Scan(&schema, &status)
	if errors.Is(e, pgx.ErrNoRows) || status != "active" {
		return nil, domain.ErrUnauthorized
	}
	if e != nil {
		return nil, e
	}
	return &Store{pool: p.root.pool, db: &familyDB{pool: p.root.pool, schema: schema, role: roleFor(id)}, familyID: id}, nil
}
func (p *Platform) Account(ctx context.Context, email, password string) (int64, int64, error) {
	key, e := emailKey(email)
	if e != nil {
		return 0, 0, domain.ErrUnauthorized
	}
	var family, owner int64
	var hash string
	e = p.root.pool.QueryRow(ctx, `select family_id,participant_id,password_hash from fq_platform.accounts where email=$1`, key).Scan(&family, &owner, &hash)
	if e != nil { // Perform a hash comparison even for an unknown account.
		// Сравнение хеша выполняется и для неизвестного аккаунта.
		hash = "$2a$10$7EqJtq98hPqEX7fNZaFWoO5VbLUYxvLm7QZV3h9fP9zxyRFPwKe3a"
	}
	valid := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
	if e != nil || !valid {
		return 0, 0, domain.ErrUnauthorized
	}
	return family, owner, nil
}
func (p *Platform) Subscription(ctx context.Context, id int64) (domain.FamilySubscription, error) {
	v := domain.FamilySubscription{FamilyID: id}
	e := p.root.pool.QueryRow(ctx, `select f.name,f.status,f.created_at,f.plan,a.email,exists(select 1 from fq_platform.payments x where x.family_id=f.id and x.starts_at<=now() and x.ends_at>now() and x.refunded_at is null) from fq_platform.families f join fq_platform.accounts a on a.family_id=f.id where f.id=$1`, id).Scan(&v.Name, &v.Status, &v.RegisteredAt, &v.Plan, &v.OwnerEmail, &v.Paid)
	if e != nil {
		return v, e
	}
	// Suspended families are readable by the operator without opening a tenant session.
	// Оператор читает реестр заблокированных семей без семейной сессии.
	var schema string
	if e = p.root.pool.QueryRow(ctx, `select schema_name from fq_platform.families where id=$1`, id).Scan(&schema); e != nil {
		return v, e
	}
	db := &familyDB{pool: p.root.pool, schema: schema, role: roleFor(id)}
	e = db.QueryRow(ctx, `select child_limit,access_until,(select count(*)::int from participants where active and role='child') from service_policy where id`).Scan(&v.ChildLimit, &v.AccessUntil, &v.ActiveChildren)
	v.Access = v.Status == "active" && (v.AccessUntil == nil || time.Now().Before(*v.AccessUntil))
	return v, e
}
func (p *Platform) Registry(ctx context.Context, offset int) ([]domain.FamilySubscription, error) {
	rows, e := p.root.pool.Query(ctx, `select id from fq_platform.families order by id limit 100 offset $1`, offset)
	if e != nil {
		return nil, e
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	out := []domain.FamilySubscription{}
	for _, id := range ids {
		v, e := p.Subscription(ctx, id)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}

type ManualPayment struct {
	FamilyID    int64     `json:"familyId"`
	Reference   string    `json:"reference"`
	AmountMinor int64     `json:"amountMinor"`
	Currency    string    `json:"currency"`
	StartsAt    time.Time `json:"startsAt"`
	EndsAt      time.Time `json:"endsAt"`
	ChildLimit  int       `json:"childLimit"`
	Operator    string    `json:"operator"`
}

func (p *Platform) RecordPayment(ctx context.Context, v ManualPayment) error {
	if v.FamilyID <= 0 || v.Reference == "" || len(v.Reference) > 200 || v.AmountMinor <= 0 || len(v.Currency) != 3 || v.ChildLimit < 1 || v.ChildLimit > 100 || !v.EndsAt.After(v.StartsAt) || v.StartsAt.After(time.Now()) || v.Operator == "" {
		return domain.ErrInvalidInput
	}
	for _, c := range v.Currency {
		if c < 'A' || c > 'Z' {
			return domain.ErrInvalidInput
		}
	}
	tx, e := p.root.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer rollback(tx)
	var schema string
	if e = tx.QueryRow(ctx, `select schema_name from fq_platform.families where id=$1 for update`, v.FamilyID).Scan(&schema); e != nil {
		return e
	}
	var prior ManualPayment
	e = tx.QueryRow(ctx, `select family_id,amount_minor,currency,starts_at,ends_at,child_limit from fq_platform.payments where reference=$1`, v.Reference).Scan(&prior.FamilyID, &prior.AmountMinor, &prior.Currency, &prior.StartsAt, &prior.EndsAt, &prior.ChildLimit)
	if e == nil {
		if prior.FamilyID == v.FamilyID && prior.AmountMinor == v.AmountMinor && prior.Currency == v.Currency && prior.StartsAt.Equal(v.StartsAt) && prior.EndsAt.Equal(v.EndsAt) && prior.ChildLimit == v.ChildLimit {
			return nil
		}
		return domain.ErrConflict
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	sch := pgx.Identifier{schema}.Sanitize()
	var children int
	var end *time.Time
	if e = tx.QueryRow(ctx, "select access_until from "+sch+".service_policy where id for update").Scan(&end); e != nil {
		return e
	}
	if e = tx.QueryRow(ctx, "select count(*) from "+sch+".participants where active and role='child'").Scan(&children); e != nil {
		return e
	}
	if children > v.ChildLimit || (end != nil && v.EndsAt.Before(*end)) {
		return domain.ErrConflict
	}
	if _, e = tx.Exec(ctx, `insert into fq_platform.payments(reference,family_id,amount_minor,currency,starts_at,ends_at,child_limit,operator) values($1,$2,$3,$4,$5,$6,$7,$8)`, v.Reference, v.FamilyID, v.AmountMinor, v.Currency, v.StartsAt, v.EndsAt, v.ChildLimit, v.Operator); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "update "+sch+".service_policy set access_until=$1,child_limit=$2", v.EndsAt, v.ChildLimit); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `update fq_platform.families set plan='family' where id=$1`, v.FamilyID); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `insert into fq_platform.audit(family_id,action,operator) values($1,'manual-payment',$2)`, v.FamilyID, v.Operator); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (p *Platform) Suspend(ctx context.Context, id int64, suspended bool, operator string) error {
	if operator == "" {
		return domain.ErrInvalidInput
	}
	state := "active"
	if suspended {
		state = "suspended"
	}
	tx, e := p.root.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer rollback(tx)
	tag, e := tx.Exec(ctx, `update fq_platform.families set status=$2 where id=$1`, id, state)
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if _, e = tx.Exec(ctx, `insert into fq_platform.audit(family_id,action,operator) values($1,$2,$3)`, id, state, operator); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// ResetAccount is an audited operator recovery, not a public password reset.
// ResetAccount — аудируемое восстановление оператором, не публичный сброс пароля.
func (p *Platform) ResetAccount(ctx context.Context, email, password, operator string) error {
	key, e := emailKey(email)
	if e != nil || len(password) < 12 || len(password) > 72 || operator == "" {
		return domain.ErrInvalidInput
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if e != nil {
		return e
	}
	tx, e := p.root.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer rollback(tx)
	var id, owner int64
	var schema string
	if e = tx.QueryRow(ctx, `select a.family_id,a.participant_id,f.schema_name from fq_platform.accounts a join fq_platform.families f on f.id=a.family_id where email=$1 for update of a`, key).Scan(&id, &owner, &schema); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `update fq_platform.accounts set password_hash=$2 where email=$1`, key, string(hash)); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "update "+pgx.Identifier{schema}.Sanitize()+".participants set session_version=session_version+1 where id=$1", owner); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `insert into fq_platform.audit(family_id,action,operator) values($1,'reset-account',$2)`, id, operator); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// ConfigureRuntime grants routing privileges without migration or billing writes.
// ConfigureRuntime выдаёт права маршрутизации без миграций и изменения оплат.
func (p *Platform) ConfigureRuntime(ctx context.Context, role string) error {
	if role == "" {
		return domain.ErrInvalidInput
	}
	tx, e := p.root.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer rollback(tx)
	name := pgx.Identifier{role}.Sanitize()
	var safe bool
	if e = tx.QueryRow(ctx, `select not rolsuper and not rolcreaterole and not rolcreatedb and not rolbypassrls from pg_roles where rolname=$1`, role).Scan(&safe); e != nil {
		return e
	}
	if !safe {
		return fmt.Errorf("runtime role must not have administrative attributes")
	}
	if _, e = tx.Exec(ctx, "alter role "+name+" noinherit;grant usage on schema fq_platform to "+name+"; grant select on fq_platform.families,fq_platform.accounts,fq_platform.payments to "+name); e != nil {
		return e
	}
	rows, e := tx.Query(ctx, `select id from fq_platform.families`)
	if e != nil {
		return e
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		if _, e = tx.Exec(ctx, "grant "+pgx.Identifier{roleFor(id)}.Sanitize()+" to "+name); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}
func (p *Platform) CheckRuntime(ctx context.Context) error {
	var safe bool
	e := p.root.pool.QueryRow(ctx, `select not rolsuper and not rolcreaterole and not rolcreatedb and not rolbypassrls and not rolinherit from pg_roles where rolname=current_user`).Scan(&safe)
	if e != nil {
		return e
	}
	if !safe {
		return fmt.Errorf("SaaS runtime requires a NOINHERIT non-administrative database role")
	}
	return nil
}

// RefundPayment records a full refund; repeating it has no additional effect.
// RefundPayment фиксирует полный возврат; повтор не меняет результат.
func (p *Platform) RefundPayment(ctx context.Context, reference, operator string) error {
	if reference == "" || operator == "" {
		return domain.ErrInvalidInput
	}
	tx, e := p.root.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer rollback(tx)
	var id int64
	if e = tx.QueryRow(ctx, `select family_id from fq_platform.payments where reference=$1`, reference).Scan(&id); e != nil {
		return e
	}
	var schema string
	if e = tx.QueryRow(ctx, `select schema_name from fq_platform.families where id=$1 for update`, id).Scan(&schema); e != nil {
		return e
	}
	tag, e := tx.Exec(ctx, `update fq_platform.payments set refunded_at=now() where reference=$1 and refunded_at is null`, reference)
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	var until time.Time
	var limit int
	e = tx.QueryRow(ctx, `select ends_at,child_limit from fq_platform.payments where family_id=$1 and refunded_at is null and starts_at<=now() and ends_at>now() order by ends_at desc,created_at desc limit 1`, id).Scan(&until, &limit)
	if errors.Is(e, pgx.ErrNoRows) {
		until = time.Now().UTC()
		limit = 0
	} else if e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "update "+pgx.Identifier{schema}.Sanitize()+".service_policy set access_until=$1,child_limit=$2", until, limit); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `insert into fq_platform.audit(family_id,action,operator) values($1,'full-refund',$2)`, id, operator); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
