package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/DujinA/spendulum/backend/internal/platform/db/gen"
)

// timeoutDBTX gives each call its own deadline. The deadline must outlive the call
// for Query and QueryRow, because rows are read after they return, so their cancel
// runs when the rows are closed or the row is scanned.
type timeoutDBTX struct {
	db      gen.DBTX
	timeout time.Duration
}

// A call inside InTx runs in its transaction.
func (t timeoutDBTX) conn(ctx context.Context) gen.DBTX {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}

	return t.db
}

func (t timeoutDBTX) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	return t.conn(ctx).Exec(ctx, sql, args...) //nolint:wrapcheck // a transparent wrapper; callers wrap
}

func (t timeoutDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)

	rows, err := t.conn(ctx).Query(ctx, sql, args...)
	if err != nil {
		cancel()

		return nil, err //nolint:wrapcheck // a transparent wrapper; callers wrap
	}

	return cancelRows{Rows: rows, cancel: cancel}, nil
}

func (t timeoutDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)

	return cancelRow{row: t.conn(ctx).QueryRow(ctx, sql, args...), cancel: cancel}
}

type cancelRows struct {
	pgx.Rows

	cancel context.CancelFunc
}

func (r cancelRows) Close() {
	r.Rows.Close()
	r.cancel()
}

type cancelRow struct {
	row    pgx.Row
	cancel context.CancelFunc
}

func (r cancelRow) Scan(dest ...any) error {
	defer r.cancel()

	return r.row.Scan(dest...) //nolint:wrapcheck // a transparent wrapper; callers wrap
}
