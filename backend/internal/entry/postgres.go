package entry

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/DujinA/spendulum/backend/internal/category"
	"github.com/DujinA/spendulum/backend/internal/platform/db/gen"
)

const (
	foreignKeyViolation = "23503"
	categoryForeignKey  = "entries_user_id_category_id_kind_fkey"
)

type Postgres struct {
	q *gen.Queries
}

func NewPostgres(q *gen.Queries) *Postgres {
	return &Postgres{q: q}
}

func (p *Postgres) DeleteExpiredKeys(ctx context.Context, userID uuid.UUID, createdBefore time.Time) error {
	err := p.q.DeleteExpiredIdempotencyKeys(ctx, gen.DeleteExpiredIdempotencyKeysParams{UserID: userID, CreatedAt: createdBefore})
	if err != nil {
		return fmt.Errorf("entry: delete expired idempotency keys: %w", err)
	}

	return nil
}

func (p *Postgres) Create(ctx context.Context, userID uuid.UUID, e Entry) error {
	err := p.q.CreateEntry(ctx, gen.CreateEntryParams{
		ID: e.ID, UserID: userID, Kind: string(e.Kind), Amount: e.Amount, Currency: e.Currency,
		CategoryID: e.CategoryID, VendorID: vendorRef(e.VendorID), EntryDate: e.EntryDate, Note: e.Note, CreatedAt: e.CreatedAt,
	})
	if isCategoryViolation(err) {
		return ErrUnknownCategory
	}

	if err != nil {
		return fmt.Errorf("entry: insert: %w", err)
	}

	return nil
}

func (p *Postgres) Update(ctx context.Context, userID uuid.UUID, e Entry) error {
	n, err := p.q.UpdateEntry(ctx, gen.UpdateEntryParams{
		Amount: e.Amount, CategoryID: e.CategoryID, VendorID: vendorRef(e.VendorID), EntryDate: e.EntryDate, Note: e.Note,
		UserID: userID, ID: e.ID,
	})
	if isCategoryViolation(err) {
		return ErrUnknownCategory
	}

	switch {
	case err != nil:
		return fmt.Errorf("entry: update: %w", err)
	case n == 0:
		return ErrNotFound
	}

	return nil
}

func (p *Postgres) Delete(ctx context.Context, userID, id uuid.UUID, at time.Time) error {
	n, err := p.q.DeleteEntry(ctx, gen.DeleteEntryParams{DeletedAt: at, UserID: userID, ID: id})

	switch {
	case err != nil:
		return fmt.Errorf("entry: delete: %w", err)
	case n == 0:
		return ErrNotFound
	}

	return nil
}

func (p *Postgres) Restore(ctx context.Context, userID, id uuid.UUID) error {
	n, err := p.q.RestoreEntry(ctx, gen.RestoreEntryParams{UserID: userID, ID: id})

	switch {
	case err != nil:
		return fmt.Errorf("entry: restore: %w", err)
	case n == 0:
		return ErrNotFound
	}

	return nil
}

func (p *Postgres) ClaimKey(ctx context.Context, userID uuid.UUID, key string, requestHash []byte, entryID uuid.UUID, at time.Time) (bool, error) {
	n, err := p.q.CreateIdempotencyKey(ctx, gen.CreateIdempotencyKeyParams{
		UserID: userID, Key: key, RequestHash: requestHash, EntryID: entryID, CreatedAt: at,
	})
	if err != nil {
		return false, fmt.Errorf("entry: insert idempotency key: %w", err)
	}

	return n == 1, nil
}

func (p *Postgres) Key(ctx context.Context, userID uuid.UUID, key string) ([]byte, uuid.UUID, error) {
	row, err := p.q.IdempotencyKey(ctx, gen.IdempotencyKeyParams{UserID: userID, Key: key})
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("entry: select idempotency key: %w", err)
	}

	return row.RequestHash, row.EntryID, nil
}

func (p *Postgres) Get(ctx context.Context, userID, id uuid.UUID) (Entry, error) {
	row, err := p.q.Entry(ctx, gen.EntryParams{UserID: userID, ID: id})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Entry{}, ErrNotFound
	case err != nil:
		return Entry{}, fmt.Errorf("entry: select: %w", err)
	}

	return fromRow(gen.EntriesRow(row)), nil
}

func (p *Postgres) List(ctx context.Context, userID uuid.UUID, after *Cursor, limit int) ([]Entry, error) {
	maxRows := int32(limit) //nolint:gosec // the handler caps a page at 100 entries

	var (
		rows []gen.EntriesRow
		err  error
	)

	if after == nil {
		rows, err = p.q.Entries(ctx, gen.EntriesParams{UserID: userID, MaxRows: maxRows})
	} else {
		var afterRows []gen.EntriesAfterRow

		afterRows, err = p.q.EntriesAfter(ctx, gen.EntriesAfterParams{
			UserID: userID, EntryDate: after.EntryDate, CreatedAt: after.CreatedAt, ID: after.ID, MaxRows: maxRows,
		})
		for _, r := range afterRows {
			rows = append(rows, gen.EntriesRow(r))
		}
	}

	if err != nil {
		return nil, fmt.Errorf("entry: select page: %w", err)
	}

	out := make([]Entry, len(rows))
	for i, r := range rows {
		out[i] = fromRow(r)
	}

	return out, nil
}

func (p *Postgres) EarliestEntryDate(ctx context.Context) (time.Time, bool, error) {
	d, err := p.q.EarliestEntryDate(ctx)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return time.Time{}, false, nil
	case err != nil:
		return time.Time{}, false, fmt.Errorf("entry: select earliest entry date: %w", err)
	}

	return d, true, nil
}

func vendorRef(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}

	return &id
}

func isCategoryViolation(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)

	return ok && pgErr.Code == foreignKeyViolation && pgErr.ConstraintName == categoryForeignKey
}

func fromRow(r gen.EntriesRow) Entry {
	e := Entry{
		ID: r.ID, Kind: category.Kind(r.Kind), Amount: r.Amount, Currency: r.Currency, CategoryID: r.CategoryID,
		CategoryName: r.CategoryName, EntryDate: r.EntryDate, Note: r.Note, CreatedAt: r.CreatedAt,
	}

	if r.VendorID != nil && r.VendorName != nil {
		e.VendorID, e.VendorName = *r.VendorID, *r.VendorName
	}

	return e
}
