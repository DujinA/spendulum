package entry_test

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
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

func TestTheSameKeyWithTheNoteEndMovedIntoTheVendorNameIsADifferentRequest(t *testing.T) {
	svc := newService(newFakeRepo())
	user := uuid.Must(uuid.NewV7())
	n := entry.NewEntry{Kind: category.Expense, Amount: 100, CategoryID: uuid.Must(uuid.NewV7()), EntryDate: march2(), Note: "Mini", VendorName: "Market"}

	if _, err := svc.Create(t.Context(), user, "k", n); err != nil {
		t.Fatalf("Create: %v", err)
	}

	n.Note, n.VendorName = "", "MiniMarket"
	if _, err := svc.Create(t.Context(), user, "k", n); !errors.Is(err, entry.ErrKeyReused) {
		t.Errorf("Create with note %q and vendor %q = %v, want ErrKeyReused", n.Note, n.VendorName, err)
	}
}

func TestAKeyStoredBeforeVendorsReplaysItsEntryForTheSameRequest(t *testing.T) {
	repo := newFakeRepo()
	n := entry.NewEntry{Kind: category.Expense, Amount: 178080, CategoryID: uuid.Must(uuid.NewV7()), EntryDate: march2(), Note: "weekly groceries"}
	stored := entry.Entry{ID: uuid.Must(uuid.NewV7()), Kind: n.Kind, Amount: n.Amount, Currency: "RSD", CategoryID: n.CategoryID, EntryDate: n.EntryDate}
	repo.entries[stored.ID] = stored
	repo.keys["k"] = fakeKey{hash: fingerprintBeforeVendors(n), entryID: stored.ID}

	got, err := newService(repo).Create(t.Context(), uuid.Must(uuid.NewV7()), "k", n)

	if err != nil || got.ID != stored.ID {
		t.Errorf("Create = %+v, %v; want the entry %v stored under the key before vendors", got, err, stored.ID)
	}
}

// fingerprintBeforeVendors is the request fingerprint as #29 wrote it, before entries had vendors.
func fingerprintBeforeVendors(n entry.NewEntry) []byte {
	h := sha256.New()
	h.Write([]byte(n.Kind))
	h.Write([]byte{0})
	_ = binary.Write(h, binary.BigEndian, n.Amount)
	h.Write(n.CategoryID[:])
	h.Write([]byte(n.EntryDate.Format(time.DateOnly)))
	h.Write([]byte{0})
	h.Write([]byte(n.Note))

	return h.Sum(nil)
}

func TestUpdateChecksTheRulesAgainstTheEntrysOwnKind(t *testing.T) {
	for name, tc := range map[string]struct {
		kind   category.Kind
		change entry.Change
		want   error
	}{
		"refund":           {category.Expense, entry.Change{Amount: -1990, CategoryID: uuid.Must(uuid.NewV7()), EntryDate: march2()}, nil},
		"income":           {category.Income, entry.Change{Amount: 1, CategoryID: uuid.Must(uuid.NewV7()), EntryDate: march2()}, nil},
		"zero expense":     {category.Expense, entry.Change{Amount: 0, CategoryID: uuid.Must(uuid.NewV7()), EntryDate: march2()}, entry.ErrExpenseZero},
		"negative income":  {category.Income, entry.Change{Amount: -500, CategoryID: uuid.Must(uuid.NewV7()), EntryDate: march2()}, entry.ErrIncomeNotPositive},
		"missing category": {category.Expense, entry.Change{Amount: 100, EntryDate: march2()}, entry.ErrCategoryRequired},
		"missing date":     {category.Expense, entry.Change{Amount: 100, CategoryID: uuid.Must(uuid.NewV7())}, entry.ErrEntryDateRequired},
		"blank vendor":     {category.Expense, entry.Change{Amount: 100, CategoryID: uuid.Must(uuid.NewV7()), EntryDate: march2(), VendorName: "?!"}, entry.ErrVendorNameBlank},
	} {
		t.Run(name, func(t *testing.T) {
			repo := newFakeRepo()
			stored := repo.add(entry.Entry{Kind: tc.kind, Amount: 700, Currency: "EUR", CategoryID: uuid.Must(uuid.NewV7()), EntryDate: march2()})

			got, err := newService(repo).Update(t.Context(), uuid.Must(uuid.NewV7()), stored.ID, tc.change)

			if !errors.Is(err, tc.want) {
				t.Fatalf("Update = %v, want %v", err, tc.want)
			}

			if tc.want != nil {
				if repo.entries[stored.ID].Amount != 700 {
					t.Errorf("a refused change was stored: %+v", repo.entries[stored.ID])
				}

				return
			}

			if got.Amount != tc.change.Amount || got.Kind != tc.kind || got.Currency != "EUR" {
				t.Errorf("Update = %+v, want amount %d on the same %s in EUR", got, tc.change.Amount, tc.kind)
			}
		})
	}
}

func TestUpdateRecordsTheVendorTheVendorsServiceAnswersAndTheNoteTrimmed(t *testing.T) {
	repo, vendors := newFakeRepo(), &fakeVendors{}
	svc := newServiceWithVendors(repo, vendors)
	stored := repo.add(entry.Entry{Kind: category.Expense, Amount: 100, CategoryID: uuid.Must(uuid.NewV7()), EntryDate: march2()})
	change := entry.Change{Amount: 250, CategoryID: stored.CategoryID, EntryDate: march2(), VendorName: " Maxi ", Note: "  fruit \n"}

	got, err := svc.Update(t.Context(), uuid.Must(uuid.NewV7()), stored.ID, change)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if len(vendors.used) != 1 || vendors.used[0] != "Maxi" || got.VendorID != vendors.ids["Maxi"] || got.Note != "fruit" {
		t.Errorf("Update = %+v after using %q, want Maxi's id %v and note %q", got, vendors.used, vendors.ids["Maxi"], "fruit")
	}

	change.VendorName = "  "
	if got, err := svc.Update(t.Context(), uuid.Must(uuid.NewV7()), stored.ID, change); err != nil || got.VendorID != uuid.Nil || len(vendors.used) != 1 {
		t.Errorf("Update with a blank vendor = %+v, %v after using %q; want no vendor and no use", got, err, vendors.used)
	}
}

func TestUpdateOfAnEntryThatIsNotThereIsNotFound(t *testing.T) {
	_, err := newService(newFakeRepo()).Update(t.Context(), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), entry.Change{
		Amount: 100, CategoryID: uuid.Must(uuid.NewV7()), EntryDate: march2(),
	})

	if !errors.Is(err, entry.ErrNotFound) {
		t.Errorf("Update = %v, want ErrNotFound", err)
	}
}

func TestDeleteHidesTheEntryAndRestoreAnswersItWithTheSameID(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo)
	user := uuid.Must(uuid.NewV7())
	stored := repo.add(entry.Entry{Kind: category.Expense, Amount: 100, CategoryID: uuid.Must(uuid.NewV7()), EntryDate: march2()})

	if err := svc.Delete(t.Context(), user, stored.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := svc.Get(t.Context(), user, stored.ID); !errors.Is(err, entry.ErrNotFound) {
		t.Errorf("Get after Delete = %v, want ErrNotFound", err)
	}

	if want := time.Date(2026, 3, 2, 9, 30, 0, 0, time.UTC); !repo.deletedAt[stored.ID].Equal(want) {
		t.Errorf("deleted at %s, want %s", repo.deletedAt[stored.ID], want)
	}

	got, err := svc.Restore(t.Context(), user, stored.ID)
	if err != nil || got.ID != stored.ID {
		t.Errorf("Restore = %+v, %v; want the entry %v", got, err, stored.ID)
	}

	if err := svc.Delete(t.Context(), user, uuid.Must(uuid.NewV7())); !errors.Is(err, entry.ErrNotFound) {
		t.Errorf("Delete of an entry that is not there = %v, want ErrNotFound", err)
	}
}

func TestARetryAfterItsEntryWasDeletedIsRefused(t *testing.T) {
	svc := newService(newFakeRepo())
	user := uuid.Must(uuid.NewV7())
	n := entry.NewEntry{Kind: category.Expense, Amount: 100, CategoryID: uuid.Must(uuid.NewV7()), EntryDate: march2()}

	created, err := svc.Create(t.Context(), user, "k", n)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Delete(t.Context(), user, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := svc.Create(t.Context(), user, "k", n); !errors.Is(err, entry.ErrKeyEntryDeleted) {
		t.Errorf("retry = %v, want ErrKeyEntryDeleted", err)
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
	entries   map[uuid.UUID]entry.Entry
	deletedAt map[uuid.UUID]time.Time
	keys      map[string]fakeKey
}

type fakeKey struct {
	hash    []byte
	entryID uuid.UUID
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{entries: map[uuid.UUID]entry.Entry{}, deletedAt: map[uuid.UUID]time.Time{}, keys: map[string]fakeKey{}}
}

func (f *fakeRepo) add(e entry.Entry) entry.Entry {
	e.ID = uuid.Must(uuid.NewV7())
	f.entries[e.ID] = e

	return e
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
	if _, deleted := f.deletedAt[id]; !ok || deleted {
		return entry.Entry{}, entry.ErrNotFound
	}

	return e, nil
}

func (f *fakeRepo) Update(ctx context.Context, userID uuid.UUID, e entry.Entry) error {
	current, err := f.Get(ctx, userID, e.ID)
	if err != nil {
		return err
	}

	current.Amount, current.CategoryID, current.VendorID, current.EntryDate, current.Note = e.Amount, e.CategoryID, e.VendorID, e.EntryDate, e.Note
	f.entries[e.ID] = current

	return nil
}

func (f *fakeRepo) Delete(_ context.Context, _ uuid.UUID, id uuid.UUID, at time.Time) error {
	if _, ok := f.entries[id]; !ok {
		return entry.ErrNotFound
	}

	if _, deleted := f.deletedAt[id]; !deleted {
		f.deletedAt[id] = at
	}

	return nil
}

func (f *fakeRepo) Restore(_ context.Context, _ uuid.UUID, id uuid.UUID) error {
	if _, ok := f.entries[id]; !ok {
		return entry.ErrNotFound
	}

	delete(f.deletedAt, id)

	return nil
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
