package entry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/DujinA/spendulum/backend/internal/category"
	"github.com/DujinA/spendulum/backend/internal/entry"
	"github.com/DujinA/spendulum/backend/internal/settings"
)

func march2() time.Time { return time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC) }

func newService(repo *fakeRepo) *entry.Service {
	return entry.NewService(entry.Deps{
		Repo:     repo,
		Tx:       fakeTx{},
		Settings: fakeSettings{settings.Settings{PrimaryCurrency: "RSD", TimeZone: "Europe/Belgrade"}},
		Now:      func() time.Time { return time.Date(2026, 3, 2, 9, 30, 0, 0, time.UTC) },
	})
}

func TestCreateAcceptsAnIncomeAboveZeroAndAnyExpenseButZero(t *testing.T) {
	for name, tc := range map[string]struct {
		kind   category.Kind
		amount int64
		want   error
	}{
		"expense":          {category.Expense, 178080, nil},
		"refund":           {category.Expense, -1990, nil},
		"income":           {category.Income, 1, nil},
		"zero expense":     {category.Expense, 0, entry.ErrExpenseZero},
		"zero income":      {category.Income, 0, entry.ErrIncomeNotPositive},
		"negative income":  {category.Income, -500, entry.ErrIncomeNotPositive},
		"unknown kind":     {category.Kind("transfer"), 100, entry.ErrUnknownKind},
		"missing category": {category.Expense, 100, entry.ErrCategoryRequired},
	} {
		t.Run(name, func(t *testing.T) {
			repo := newFakeRepo()
			n := entry.NewEntry{Kind: tc.kind, Amount: tc.amount, CategoryID: uuid.Must(uuid.NewV7()), EntryDate: march2()}
			if name == "missing category" {
				n.CategoryID = uuid.Nil
			}

			_, err := newService(repo).Create(t.Context(), uuid.Must(uuid.NewV7()), "key-"+name, n)

			if !errors.Is(err, tc.want) {
				t.Fatalf("Create = %v, want %v", err, tc.want)
			}

			if stored := len(repo.entries) == 1; stored != (tc.want == nil) {
				t.Errorf("stored an entry: %v, want %v", stored, tc.want == nil)
			}
		})
	}
}

func TestCreateRefusesAnEntryWithoutAnEntryDate(t *testing.T) {
	_, err := newService(newFakeRepo()).Create(t.Context(), uuid.Must(uuid.NewV7()), "k", entry.NewEntry{
		Kind: category.Expense, Amount: 100, CategoryID: uuid.Must(uuid.NewV7()),
	})

	if !errors.Is(err, entry.ErrEntryDateRequired) {
		t.Errorf("Create = %v, want ErrEntryDateRequired", err)
	}
}

func TestCreateRecordsThePrimaryCurrencyAndTheNoteTrimmed(t *testing.T) {
	repo := newFakeRepo()

	got, err := newService(repo).Create(t.Context(), uuid.Must(uuid.NewV7()), "k", entry.NewEntry{
		Kind: category.Expense, Amount: 178080, CategoryID: uuid.Must(uuid.NewV7()), EntryDate: march2(), Note: "  weekly shop \n",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got.Currency != "RSD" || got.Note != "weekly shop" || got.Amount != 178080 || !got.EntryDate.Equal(march2()) {
		t.Errorf("entry = %+v, want 178080 RSD on 2026-03-02 with note %q", got, "weekly shop")
	}
}

type fakeRepo struct {
	entries map[uuid.UUID]entry.Entry
	keys    map[string]fakeKey
}

type fakeKey struct {
	hash    []byte
	entryID uuid.UUID
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{entries: map[uuid.UUID]entry.Entry{}, keys: map[string]fakeKey{}}
}

func (f *fakeRepo) DeleteExpiredKeys(context.Context, uuid.UUID, time.Time) error { return nil }

func (f *fakeRepo) Create(_ context.Context, _ uuid.UUID, e entry.Entry) error {
	f.entries[e.ID] = e

	return nil
}

func (f *fakeRepo) ClaimKey(_ context.Context, _ uuid.UUID, key string, hash []byte, entryID uuid.UUID, _ time.Time) (bool, error) {
	if _, ok := f.keys[key]; ok {
		return false, nil
	}

	f.keys[key] = fakeKey{hash: hash, entryID: entryID}

	return true, nil
}

func (f *fakeRepo) Key(_ context.Context, _ uuid.UUID, key string) ([]byte, uuid.UUID, error) {
	k := f.keys[key]

	return k.hash, k.entryID, nil
}

func (f *fakeRepo) Get(_ context.Context, _ uuid.UUID, id uuid.UUID) (entry.Entry, error) {
	e, ok := f.entries[id]
	if !ok {
		return entry.Entry{}, entry.ErrNotFound
	}

	return e, nil
}

func (f *fakeRepo) EarliestEntryDate(context.Context) (time.Time, bool, error) {
	return time.Time{}, false, nil
}

func (f *fakeRepo) List(context.Context, uuid.UUID, *entry.Cursor, int) ([]entry.Entry, error) {
	return nil, nil
}

type fakeTx struct{}

func (fakeTx) InTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

type fakeSettings struct{ s settings.Settings }

func (f fakeSettings) Get(context.Context, uuid.UUID) (settings.Settings, error) { return f.s, nil }
