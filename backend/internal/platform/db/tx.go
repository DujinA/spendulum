package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type txKey struct{}

// Every query made with fn's ctx, through any adapter, joins the transaction, as does a nested InTx (ADR 0010).
func (d *DB) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}

	beginCtx, cancel := context.WithTimeout(ctx, d.timeout)
	tx, err := d.pool.Begin(beginCtx)

	cancel()

	if err != nil {
		return fmt.Errorf("db: begin: %w", err)
	}

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return errors.Join(err, d.rollback(ctx, tx))
	}

	commitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), d.timeout)
	defer cancel()

	if err := tx.Commit(commitCtx); err != nil {
		return fmt.Errorf("db: commit: %w", err)
	}

	return nil
}

func (d *DB) rollback(ctx context.Context, tx pgx.Tx) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), d.timeout)
	defer cancel()

	if err := tx.Rollback(ctx); err != nil {
		return fmt.Errorf("db: rollback: %w", err)
	}

	return nil
}
