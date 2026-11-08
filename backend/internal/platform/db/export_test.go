package db

import "context"

// ExecForTest runs sql through the same timeout-bound connection as the generated queries.
func (d *DB) ExecForTest(ctx context.Context, sql string) error {
	_, err := d.dbtx().Exec(ctx, sql)

	return err
}

// QueryForTest runs sql and reads every row, as generated :many queries do.
func (d *DB) QueryForTest(ctx context.Context, sql string) error {
	rows, err := d.dbtx().Query(ctx, sql)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		if _, err := rows.Values(); err != nil {
			return err
		}
	}

	return rows.Err()
}

// QueryRowForTest runs sql and scans its one row, as generated :one queries do.
func (d *DB) QueryRowForTest(ctx context.Context, sql string) error {
	var v any

	return d.dbtx().QueryRow(ctx, sql).Scan(&v)
}
