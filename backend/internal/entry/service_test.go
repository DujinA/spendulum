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
	return newServiceWithVendors(repo, &fakeVendors{})
}

func newServiceWithVendors(repo *fakeRepo, vendors *fakeVendors) *entry.Service {
	return entry.NewService(entry.Deps{
		Repo:     repo,
		Vendors:  vendors,
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

func TestCreateRecordsTheVendorTheVendorsServiceAnswersForTheName(t *testing.T) {
	repo, vendors := newFakeRepo(), &fakeVendors{}
	svc := newServiceWithVendors(repo, vendors)
	n := entry.NewEntry{Kind: category.Expense, Amount: 100, CategoryID: uuid.Must(uuid.NewV7()), EntryDate: march2()}

	withVendor := n
	withVendor.VendorName = "  Kafetrija "

	got, err := svc.Create(t.Context(), uuid.Must(uuid.NewV7()), "k1", withVendor)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if len(vendors.used) != 1 || vendors.used[0] != "Kafetrija" || got.VendorID != vendors.ids["Kafetrija"] {
		t.Errorf("used %q and recorded vendor %v, want Kafetrija's id %v", vendors.used, got.VendorID, vendors.ids["Kafetrija"])
	}

	none, err := svc.Create(t.Context(), uuid.Must(uuid.NewV7()), "k2", n)
	if err != nil || none.VendorID != uuid.Nil || len(vendors.used) != 1 {
		t.Errorf("Create without a vendor = %+v, %v after using %q; want no vendor and no use", none, err, vendors.used)
	}
}

func TestCreateRefusesAVendorNameWithoutALetterOrDigit(t *testing.T) {
	repo, vendors := newFakeRepo(), &fakeVendors{}

	_, err := newServiceWithVendors(repo, vendors).Create(t.Context(), uuid.Must(uuid.NewV7()), "k", entry.NewEntry{
		Kind: category.Expense, Amount: 100, CategoryID: uuid.Must(uuid.NewV7()), EntryDate: march2(), VendorName: " -!- ",
	})

	if !errors.Is(err, entry.ErrVendorNameBlank) || len(vendors.used) != 0 || len(repo.entries) != 0 {
		t.Errorf("Create = %v after using %q, want ErrVendorNameBlank and nothing stored", err, vendors.used)
	}
}

func TestCreateTakesOnlyThePrimaryCurrency(t *testing.T) {
	for currency, want := range map[string]error{"": nil, "RSD": nil, "EUR": entry.ErrCurrencyNotPrimary} {
		repo := newFakeRepo()

		got, err := newService(repo).Create(t.Context(), uuid.Must(uuid.NewV7()), "k", entry.NewEntry{
			Kind: category.Expense, Amount: 100, Currency: currency, CategoryID: uuid.Must(uuid.NewV7()), EntryDate: march2(),
		})

		if !errors.Is(err, want) || (want == nil && got.Currency != "RSD") {
			t.Errorf("Create in %q = %+v, %v; want %v", currency, got, err, want)
		}
	}
}

func TestTheSameKeyWithAnotherVendorIsADifferentRequest(t *testing.T) {
	svc := newService(newFakeRepo())
	user := uuid.Must(uuid.NewV7())
	n := entry.NewEntry{Kind: category.Expense, Amount: 100, CategoryID: uuid.Must(uuid.NewV7()), EntryDate: march2(), VendorName: "Maxi"}

	if _, err := svc.Create(t.Context(), user, "k", n); err != nil {
		t.Fatalf("Create: %v", err)
	}

	n.VendorName = "Lidl"
	if _, err := svc.Create(t.Context(), user, "k", n); !errors.Is(err, entry.ErrKeyReused) {
		t.Errorf("Create with another vendor = %v, want ErrKeyReused", err)
	}
}

type fakeVendors struct {
	used []string
	ids  map[string]uuid.UUID
}

func (f *fakeVendors) Use(_ context.Context, _ uuid.UUID, name string) (uuid.UUID, error) {
	if f.ids == nil {
		f.ids = map[string]uuid.UUID{}
	}

	f.used = append(f.used, name)
	if _, ok := f.ids[name]; !ok {
		f.ids[name] = uuid.Must(uuid.NewV7())
	}

	return f.ids[name], nil
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

func (f *fakeRepo) List(context.Context, uuid.UUID, *entry.Cursor, int) ([]entry.Entry, error) {
	return nil, nil
}

type fakeTx struct{}

func (fakeTx) InTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

type fakeSettings struct{ s settings.Settings }

func (f fakeSettings) Get(context.Context, uuid.UUID) (settings.Settings, error) { return f.s, nil }
