package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/DujinA/spendulum/backend/internal/platform/db"
	"github.com/DujinA/spendulum/backend/internal/platform/db/dbtest"
	"github.com/DujinA/spendulum/backend/internal/platform/db/gen"
)

func TestInTxCommitsEveryWriteWhenFnSucceeds(t *testing.T) {
	d := open(t, dbtest.NewDatabase(t), time.Second)

	err := d.InTx(t.Context(), func(ctx context.Context) error {
		return createUser(ctx, d, "ana@example.test")
	})
	if err != nil {
		t.Fatalf("InTx: %v", err)
	}

	if _, err := d.Queries().UserByEmail(t.Context(), "ana@example.test"); err != nil {
		t.Errorf("user written in the transaction is missing: %v", err)
	}
}

func TestInTxRollsBackEveryWriteWhenFnFails(t *testing.T) {
	d := open(t, dbtest.NewDatabase(t), time.Second)
	boom := errors.New("boom")

	err := d.InTx(t.Context(), func(ctx context.Context) error {
		if err := createUser(ctx, d, "ana@example.test"); err != nil {
			return err
		}

		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("InTx error = %v, want %v", err, boom)
	}

	if _, err := d.Queries().UserByEmail(t.Context(), "ana@example.test"); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("UserByEmail after rollback: %v, want no rows", err)
	}
}

func TestNestedInTxJoinsTheOuterTransaction(t *testing.T) {
	d := open(t, dbtest.NewDatabase(t), time.Second)
	boom := errors.New("boom")

	err := d.InTx(t.Context(), func(ctx context.Context) error {
		if err := d.InTx(ctx, func(ctx context.Context) error { return createUser(ctx, d, "ana@example.test") }); err != nil {
			return err
		}

		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("InTx error = %v, want %v", err, boom)
	}

	if _, err := d.Queries().UserByEmail(t.Context(), "ana@example.test"); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("the inner write survived the outer rollback: %v", err)
	}
}

func createUser(ctx context.Context, d *db.DB, email string) error {
	return d.Queries().CreateUser(ctx, gen.CreateUserParams{ID: uuid.Must(uuid.NewV7()), Email: email, PasswordHash: "x"})
}
