package vendors_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/DujinA/spendulum/backend/internal/category"
	"github.com/DujinA/spendulum/backend/internal/entry"
	"github.com/DujinA/spendulum/backend/internal/platform/db"
	"github.com/DujinA/spendulum/backend/internal/platform/db/dbtest"
	"github.com/DujinA/spendulum/backend/internal/platform/namekey"
	"github.com/DujinA/spendulum/backend/internal/vendors"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

type fixture struct {
	repo *vendors.Postgres
	d    *db.DB
}

func newFixture(t *testing.T) fixture {
	t.Helper()

	d := dbtest.Open(t)

	return fixture{repo: vendors.NewPostgres(d.Queries()), d: d}
}

func (f fixture) use(t *testing.T, userID uuid.UUID, name string) uuid.UUID {
	t.Helper()

	id, err := f.repo.Use(t.Context(), userID, vendors.Vendor{ID: uuid.Must(uuid.NewV7()), Name: name, Key: namekey.Of(name)})
	if err != nil {
		t.Fatalf("Use(%q): %v", name, err)
	}

	return id
}

func (f fixture) list(t *testing.T, userID uuid.UUID, kind category.Kind) map[string]vendors.Vendor {
	t.Helper()

	all, err := f.repo.List(t.Context(), userID, kind)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	byName := map[string]vendors.Vendor{}
	for _, v := range all {
		byName[v.Name] = v
	}

	return byName
}

func (f fixture) category(t *testing.T, userID uuid.UUID, kind category.Kind, name string) uuid.UUID {
	t.Helper()

	id := uuid.Must(uuid.NewV7())
	if err := category.NewPostgres(f.d.Queries()).Create(t.Context(), userID, category.Category{ID: id, Kind: kind, Name: name}); err != nil {
		t.Fatalf("create category: %v", err)
	}

	return id
}

type fakeEntry struct {
	kind      category.Kind
	category  uuid.UUID
	vendor    uuid.UUID
	currency  string
	date      string
	createdAt time.Time
}

func (f fixture) entry(t *testing.T, userID uuid.UUID, e fakeEntry) {
	t.Helper()

	date, err := time.Parse(time.DateOnly, e.date)
	if err != nil {
		t.Fatal(err)
	}

	err = entry.NewPostgres(f.d.Queries()).Create(t.Context(), userID, entry.Entry{
		ID: uuid.Must(uuid.NewV7()), Kind: e.kind, Amount: 1000, Currency: e.currency, CategoryID: e.category,
		VendorID: e.vendor, EntryDate: date, CreatedAt: e.createdAt,
	})
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
}

func TestUseCreatesAVendorOnceAndAfterwardsAnswersTheOneWithTheSameKey(t *testing.T) {
	f := newFixture(t)
	user := dbtest.NewUser(t, f.d.Queries())

	maxi := f.use(t, user, "Maxi")

	if again := f.use(t, user, "MAXI"); again != maxi {
		t.Errorf("Use(MAXI) = %v, want Maxi's %v", again, maxi)
	}

	got := f.list(t, user, category.Expense)
	if len(got) != 1 || got["Maxi"].ID != maxi || got["Maxi"].Key != "maxi" {
		t.Errorf("List = %+v, want only Maxi with key maxi", got)
	}
}

func TestUseUnarchivesTheVendorWithTheKey(t *testing.T) {
	f := newFixture(t)
	user := dbtest.NewUser(t, f.d.Queries())
	knjizara := f.use(t, user, "Stara Knjižara")

	if err := f.repo.Archive(t.Context(), user, knjizara); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	if !f.list(t, user, category.Expense)["Stara Knjižara"].Archived {
		t.Fatal("the vendor is not archived")
	}

	if again := f.use(t, user, "stara knjizara"); again != knjizara {
		t.Errorf("Use = %v, want the archived vendor %v", again, knjizara)
	}

	if f.list(t, user, category.Expense)["Stara Knjižara"].Archived {
		t.Error("Use left the vendor archived")
	}
}

func TestListCountsEntriesOfTheKindAndTakesTheLatestByEntryDateThenCreationTime(t *testing.T) {
	f := newFixture(t)
	user := dbtest.NewUser(t, f.d.Queries())
	groceries := f.category(t, user, category.Expense, "Groceries")
	eatingOut := f.category(t, user, category.Expense, "Eating out")
	gifts := f.category(t, user, category.Income, "Gifts")
	maxi := f.use(t, user, "Maxi")
	f.use(t, user, "Lidl")
	at := func(hour int) time.Time { return time.Date(2026, 3, 5, hour, 0, 0, 0, time.UTC) }

	f.entry(t, user, fakeEntry{category.Expense, eatingOut, maxi, "EUR", "2026-03-02", at(9)})
	f.entry(t, user, fakeEntry{category.Expense, groceries, maxi, "RSD", "2026-03-02", at(8)})
	f.entry(t, user, fakeEntry{category.Expense, groceries, maxi, "RSD", "2026-03-01", at(12)})
	f.entry(t, user, fakeEntry{category.Income, gifts, maxi, "RSD", "2026-03-04", at(7)})
	f.entry(t, user, fakeEntry{category.Expense, groceries, uuid.Nil, "RSD", "2026-03-03", at(7)})

	expenses := f.list(t, user, category.Expense)

	want := vendors.LatestUse{CategoryID: eatingOut, CategoryName: "Eating out", Currency: "EUR"}
	if got := expenses["Maxi"]; got.UseCount != 3 || got.Latest == nil || *got.Latest != want {
		t.Errorf("Maxi for expenses = %+v (latest %+v), want 3 uses, latest %+v", got, got.Latest, want)
	}

	if got := expenses["Lidl"]; got.UseCount != 0 || got.Latest != nil {
		t.Errorf("Lidl = %+v, want no use and no latest entry", got)
	}

	income := f.list(t, user, category.Income)
	if got := income["Maxi"]; got.UseCount != 1 || got.Latest == nil || got.Latest.CategoryName != "Gifts" {
		t.Errorf("Maxi for income = %+v (latest %+v), want 1 use in Gifts", got, got.Latest)
	}
}

func TestOneUserNeverSeesNorChangesAnotherUsersVendors(t *testing.T) {
	f := newFixture(t)
	a, b := dbtest.NewUser(t, f.d.Queries()), dbtest.NewUser(t, f.d.Queries())
	aMaxi := f.use(t, a, "Maxi")

	if err := f.repo.Archive(t.Context(), b, aMaxi); !errors.Is(err, vendors.ErrNotFound) {
		t.Errorf("B's Archive of A's vendor = %v, want ErrNotFound", err)
	}

	if got := f.list(t, b, category.Expense); len(got) != 0 {
		t.Errorf("B lists %+v, want none of A's vendors", got)
	}

	if err := f.repo.Archive(t.Context(), a, aMaxi); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	if bMaxi := f.use(t, b, "Maxi"); bMaxi == aMaxi {
		t.Fatal("B's Use answered A's vendor")
	}

	if got := f.list(t, a, category.Expense); len(got) != 1 || !got["Maxi"].Archived || got["Maxi"].ID != aMaxi {
		t.Errorf("A lists %+v, want only A's Maxi, still archived", got)
	}
}
