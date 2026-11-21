package entry_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/DujinA/spendulum/backend/internal/category"
	"github.com/DujinA/spendulum/backend/internal/entry"
	"github.com/DujinA/spendulum/backend/internal/platform/db"
	"github.com/DujinA/spendulum/backend/internal/platform/db/dbtest"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

type fixture struct {
	repo *entry.Postgres
	d    *db.DB
}

func newFixture(t *testing.T) fixture {
	t.Helper()

	d := dbtest.Open(t)

	return fixture{repo: entry.NewPostgres(d.Queries()), d: d}
}

func (f fixture) category(t *testing.T, userID uuid.UUID, kind category.Kind, name string) uuid.UUID {
	t.Helper()

	id := uuid.Must(uuid.NewV7())
	if err := category.NewPostgres(f.d.Queries()).Create(t.Context(), userID, category.Category{ID: id, Kind: kind, Name: name}); err != nil {
		t.Fatalf("create category: %v", err)
	}

	return id
}

func (f fixture) entry(t *testing.T, userID, categoryID uuid.UUID, date string, createdAt time.Time) entry.Entry {
	t.Helper()

	e := entry.Entry{
		ID: uuid.Must(uuid.NewV7()), Kind: category.Expense, Amount: 1000, Currency: "RSD",
		CategoryID: categoryID, EntryDate: day(t, date), Note: "fake note", CreatedAt: createdAt,
	}
	if err := f.repo.Create(t.Context(), userID, e); err != nil {
		t.Fatalf("Create: %v", err)
	}

	return e
}

func day(t *testing.T, date string) time.Time {
	t.Helper()

	d, err := time.Parse(time.DateOnly, date)
	if err != nil {
		t.Fatal(err)
	}

	return d
}

func TestCreatedEntryIsReadBackWithItsCategoryName(t *testing.T) {
	f := newFixture(t)
	user := dbtest.NewUser(t, f.d.Queries())
	cat := f.category(t, user, category.Expense, "Groceries")
	created := time.Date(2026, 3, 2, 9, 30, 0, 0, time.UTC)

	want := f.entry(t, user, cat, "2026-03-02", created)

	got, err := f.repo.Get(t.Context(), user, want.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	want.CategoryName = "Groceries"
	if got.ID != want.ID || got.Kind != want.Kind || got.Amount != want.Amount || got.Currency != want.Currency ||
		got.CategoryID != want.CategoryID || got.CategoryName != want.CategoryName || !got.EntryDate.Equal(want.EntryDate) ||
		got.Note != want.Note || !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("Get = %+v, want %+v", got, want)
	}
}

func TestEntriesAreListedNewestEntryDateFirstThenNewestCreatedFirst(t *testing.T) {
	f := newFixture(t)
	user := dbtest.NewUser(t, f.d.Queries())
	cat := f.category(t, user, category.Expense, "Groceries")
	at := func(hour int) time.Time { return time.Date(2026, 3, 5, hour, 0, 0, 0, time.UTC) }

	older := f.entry(t, user, cat, "2026-03-01", at(12))
	firstOnTheSecond := f.entry(t, user, cat, "2026-03-02", at(8))
	newest := f.entry(t, user, cat, "2026-03-03", at(7))
	laterOnTheSecond := f.entry(t, user, cat, "2026-03-02", at(9))
	want := []uuid.UUID{newest.ID, laterOnTheSecond.ID, firstOnTheSecond.ID, older.ID}

	all, err := f.repo.List(t.Context(), user, nil, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	assertIDs(t, all, want)

	last := all[1]
	after, err := f.repo.List(t.Context(), user, &entry.Cursor{EntryDate: last.EntryDate, CreatedAt: last.CreatedAt, ID: last.ID}, 10)
	if err != nil {
		t.Fatalf("List after: %v", err)
	}

	assertIDs(t, after, want[2:])
}

func TestOneUserNeverReadsAnotherUsersEntries(t *testing.T) {
	f := newFixture(t)
	a, b := dbtest.NewUser(t, f.d.Queries()), dbtest.NewUser(t, f.d.Queries())
	aEntry := f.entry(t, a, f.category(t, a, category.Expense, "Groceries"), "2026-03-02", time.Now())

	if _, err := f.repo.Get(t.Context(), b, aEntry.ID); !errors.Is(err, entry.ErrNotFound) {
		t.Errorf("B's Get of A's entry = %v, want ErrNotFound", err)
	}

	if got, err := f.repo.List(t.Context(), b, nil, 10); err != nil || len(got) != 0 {
		t.Errorf("B's List = %+v, %v; want none of A's entries", got, err)
	}

	bEntry := f.entry(t, b, f.category(t, b, category.Expense, "Groceries"), "2026-03-02", time.Now())

	if got, err := f.repo.List(t.Context(), a, nil, 10); err != nil || len(got) != 1 || got[0].ID != aEntry.ID {
		t.Errorf("A's List = %+v, %v; want only A's entry", got, err)
	}

	if _, err := f.repo.Get(t.Context(), a, bEntry.ID); !errors.Is(err, entry.ErrNotFound) {
		t.Errorf("A's Get of B's entry = %v, want ErrNotFound", err)
	}
}

// The startup rate backfill needs rates for every user's entries, so this one read is across users (#35).
func TestEarliestEntryDateIsTheEarliestOfAnyUser(t *testing.T) {
	f := newFixture(t)
	svc := entry.NewService(entry.Deps{Repo: f.repo})

	if _, ok, err := svc.EarliestEntryDate(t.Context()); ok || err != nil {
		t.Errorf("EarliestEntryDate with no entries = %v, %v; want none", ok, err)
	}

	a, b := dbtest.NewUser(t, f.d.Queries()), dbtest.NewUser(t, f.d.Queries())
	aCat, bCat := f.category(t, a, category.Expense, "Groceries"), f.category(t, b, category.Expense, "Groceries")
	f.entry(t, a, aCat, "2026-03-05", time.Now())
	f.entry(t, b, bCat, "2026-02-10", time.Now())
	f.entry(t, a, aCat, "2026-01-20", time.Now())
	f.entry(t, b, bCat, "2026-04-01", time.Now())

	if got, ok, err := svc.EarliestEntryDate(t.Context()); !ok || err != nil || !got.Equal(day(t, "2026-01-20")) {
		t.Errorf("EarliestEntryDate = %s, %v, %v; want A's 2026-01-20", got, ok, err)
	}

	f.entry(t, b, bCat, "2025-12-31", time.Now())

	if got, _, _ := svc.EarliestEntryDate(t.Context()); !got.Equal(day(t, "2025-12-31")) {
		t.Errorf("EarliestEntryDate = %s, want B's 2025-12-31", got)
	}
}

func TestAnEntryNeedsACategoryOfTheSameUserAndKind(t *testing.T) {
	f := newFixture(t)
	a, b := dbtest.NewUser(t, f.d.Queries()), dbtest.NewUser(t, f.d.Queries())
	salary := f.category(t, a, category.Income, "Salary")
	bGroceries := f.category(t, b, category.Expense, "Groceries")

	for name, categoryID := range map[string]uuid.UUID{
		"another kind's category": salary,
		"another user's category": bGroceries,
		"no such category":        uuid.Must(uuid.NewV7()),
	} {
		t.Run(name, func(t *testing.T) {
			e := entry.Entry{
				ID: uuid.Must(uuid.NewV7()), Kind: category.Expense, Amount: 1000, Currency: "RSD",
				CategoryID: categoryID, EntryDate: day(t, "2026-03-02"), CreatedAt: time.Now(),
			}

			if err := f.repo.Create(t.Context(), a, e); !errors.Is(err, entry.ErrUnknownCategory) {
				t.Errorf("Create = %v, want ErrUnknownCategory", err)
			}
		})
	}
}

func TestIdempotencyKeysAreScopedByUserAndFreedWhenExpired(t *testing.T) {
	f := newFixture(t)
	a, b := dbtest.NewUser(t, f.d.Queries()), dbtest.NewUser(t, f.d.Queries())
	claimedAt := time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC)
	aEntry := f.entry(t, a, f.category(t, a, category.Expense, "Groceries"), "2026-03-02", claimedAt)
	bEntry := f.entry(t, b, f.category(t, b, category.Expense, "Groceries"), "2026-03-02", claimedAt)

	claim := func(user, entryID uuid.UUID, hash string) bool {
		t.Helper()

		ok, err := f.repo.ClaimKey(t.Context(), user, "the-key", []byte(hash), entryID, claimedAt)
		if err != nil {
			t.Fatalf("ClaimKey: %v", err)
		}

		return ok
	}

	if !claim(a, aEntry.ID, "a") || claim(a, aEntry.ID, "a") {
		t.Fatal("A could not claim the key once, or claimed it twice")
	}

	if !claim(b, bEntry.ID, "b") {
		t.Error("B could not claim a key that only A holds")
	}

	if hash, id, err := f.repo.Key(t.Context(), b, "the-key"); err != nil || !bytes.Equal(hash, []byte("b")) || id != bEntry.ID {
		t.Errorf("B's Key = %q, %v, %v; want B's own", hash, id, err)
	}

	if err := f.repo.DeleteExpiredKeys(t.Context(), a, claimedAt); err != nil {
		t.Fatalf("DeleteExpiredKeys: %v", err)
	}

	if !claim(a, aEntry.ID, "a again") {
		t.Error("A could not claim its key again after it expired")
	}

	if claim(b, bEntry.ID, "b") {
		t.Error("A's expiry freed B's key")
	}
}

func assertIDs(t *testing.T, got []entry.Entry, want []uuid.UUID) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("listed %d entries, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i].ID != want[i] {
			t.Errorf("entry %d = %v (%s), want %v", i, got[i].ID, got[i].EntryDate.Format(time.DateOnly), want[i])
		}
	}
}
