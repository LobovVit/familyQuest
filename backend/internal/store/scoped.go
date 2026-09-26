package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// sqlDB keeps every query on the selected family's connection boundary.
// sqlDB удерживает каждый запрос в границе выбранной семьи.
type sqlDB interface {
	Begin(context.Context) (pgx.Tx, error)
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type familyDB struct {
	pool         *pgxpool.Pool
	schema, role string
}

func (d *familyDB) Begin(ctx context.Context) (pgx.Tx, error) { return d.BeginTx(ctx, pgx.TxOptions{}) }
func (d *familyDB) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	tx, e := d.pool.BeginTx(ctx, opts)
	if e != nil {
		return nil, e
	}
	// Values are catalog-generated identifiers, never client input.
	// Идентификаторы создаёт каталог, они не принимаются от клиента.
	if _, e = tx.Exec(ctx, "SET LOCAL ROLE "+pgx.Identifier{d.role}.Sanitize()); e == nil {
		_, e = tx.Exec(ctx, "select set_config('search_path',$1,true)", pgx.Identifier{d.schema}.Sanitize()+",pg_catalog")
	}
	if e != nil {
		rollback(tx)
		return nil, e
	}
	return tx, nil
}
func rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}
func (d *familyDB) Exec(ctx context.Context, q string, a ...any) (pgconn.CommandTag, error) {
	tx, e := d.Begin(ctx)
	if e != nil {
		return pgconn.CommandTag{}, e
	}
	defer rollback(tx)
	tag, e := tx.Exec(ctx, q, a...)
	if e == nil {
		e = tx.Commit(ctx)
	}
	return tag, e
}
func (d *familyDB) Query(ctx context.Context, q string, a ...any) (pgx.Rows, error) {
	tx, e := d.Begin(ctx)
	if e != nil {
		return nil, e
	}
	rows, e := tx.Query(ctx, q, a...)
	if e != nil {
		rollback(tx)
		return nil, e
	}
	return &familyRows{Rows: rows, tx: tx, ctx: ctx}, nil
}

type familyRows struct {
	pgx.Rows
	tx   pgx.Tx
	ctx  context.Context
	done bool
	err  error
}

func (r *familyRows) Close() {
	if r.done {
		return
	}
	r.done = true
	r.Rows.Close()
	r.err = r.Rows.Err()
	if r.err == nil {
		r.err = r.tx.Commit(r.ctx)
	}
	rollback(r.tx)
}
func (r *familyRows) Next() bool {
	if r.done {
		return false
	}
	ok := r.Rows.Next()
	if !ok {
		r.Close()
	}
	return ok
}
func (r *familyRows) Err() error {
	if r.err != nil {
		return r.err
	}
	return r.Rows.Err()
}

type familyRow struct {
	db  *familyDB
	ctx context.Context
	q   string
	a   []any
}

func (d *familyDB) QueryRow(ctx context.Context, q string, a ...any) pgx.Row {
	return &familyRow{d, ctx, q, a}
}
func (r *familyRow) Scan(v ...any) error {
	tx, e := r.db.Begin(r.ctx)
	if e != nil {
		return e
	}
	defer rollback(tx)
	if e = tx.QueryRow(r.ctx, r.q, r.a...).Scan(v...); e != nil {
		return e
	}
	return tx.Commit(r.ctx)
}
func schemaFor(id int64) string { return fmt.Sprintf("fq_family_%d", id) }
func roleFor(id int64) string   { return fmt.Sprintf("fq_family_%d_runtime", id) }
