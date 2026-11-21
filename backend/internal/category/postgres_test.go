package category_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/DujinA/spendulum/backend/internal/category"
	"github.com/DujinA/spendulum/backend/internal/platform/db/dbtest"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

func TestOneUserNeverListsAnotherUsersCategories(t *testing.T) {
	d := dbtest.Open(t)
	repo := category.NewPostgres(d.Queries())
	a, b := dbtest.NewUser(t, d.Queries()), dbtest.NewUser(t, d.Queries())

	aCat := category.Category{ID: uuid.Must(uuid.NewV7()), Kind: category.Expense, Name: "Groceries", Position: 0}
	bCat := category.Category{ID: uuid.Must(uuid.NewV7()), Kind: category.Income, Name: "Salary", Position: 0}

	if err := repo.Create(t.Context(), a, aCat); err != nil {
		t.Fatalf("Create for A: %v", err)
	}

	if err := repo.Create(t.Context(), b, bCat); err != nil {
		t.Fatalf("Create for B: %v", err)
	}

	got, err := repo.List(t.Context(), b)
	if err != nil {
		t.Fatalf("List for B: %v", err)
	}

	if len(got) != 1 || got[0] != bCat {
		t.Errorf("B lists %+v, want only B's %+v", got, bCat)
	}
}

func TestUnarchivedCategoriesLeaveOutArchivedOnesAndKeepTheManualOrder(t *testing.T) {
	d := dbtest.Open(t)
	repo := category.NewPostgres(d.Queries())
	user := dbtest.NewUser(t, d.Queries())

	rent := category.Category{ID: uuid.Must(uuid.NewV7()), Kind: category.Expense, Name: "Rent", Position: 2}
	archived := category.Category{ID: uuid.Must(uuid.NewV7()), Kind: category.Expense, Name: "Old hobby", Position: 1, Archived: true}
	food := category.Category{ID: uuid.Must(uuid.NewV7()), Kind: category.Expense, Name: "Food", Position: 0}
	salary := category.Category{ID: uuid.Must(uuid.NewV7()), Kind: category.Income, Name: "Salary", Position: 0}

	for _, c := range []category.Category{salary, rent, archived, food} {
		if err := repo.Create(t.Context(), user, c); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	got, err := repo.ListUnarchived(t.Context(), user)
	if err != nil {
		t.Fatalf("ListUnarchived: %v", err)
	}

	want := []category.Category{food, rent, salary}
	if len(got) != len(want) {
		t.Fatalf("ListUnarchived = %+v, want %+v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ListUnarchived[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestOneUserNeverListsAnotherUsersUnarchivedCategories(t *testing.T) {
	d := dbtest.Open(t)
	repo := category.NewPostgres(d.Queries())
	a, b := dbtest.NewUser(t, d.Queries()), dbtest.NewUser(t, d.Queries())

	if err := repo.Create(t.Context(), a, category.Category{ID: uuid.Must(uuid.NewV7()), Kind: category.Expense, Name: "Groceries"}); err != nil {
		t.Fatalf("Create for A: %v", err)
	}

	got, err := repo.ListUnarchived(t.Context(), b)
	if err != nil || len(got) != 0 {
		t.Errorf("B's ListUnarchived = %+v, %v; want none of A's", got, err)
	}
}

func TestOneUserNeverGetsChangesReordersOrDeletesAnotherUsersCategory(t *testing.T) {
	d := dbtest.Open(t)
	repo := category.NewPostgres(d.Queries())
	a, b := dbtest.NewUser(t, d.Queries()), dbtest.NewUser(t, d.Queries())
	anas := category.Category{ID: uuid.Must(uuid.NewV7()), Kind: category.Expense, Name: "Kafići", Note: "Coffee", Position: 3}

	if err := repo.Create(t.Context(), a, anas); err != nil {
		t.Fatalf("Create for A: %v", err)
	}

	if _, err := repo.Get(t.Context(), b, anas.ID); !errors.Is(err, category.ErrNotFound) {
		t.Errorf("B's Get = %v, want ErrNotFound", err)
	}

	if _, err := repo.ByNameKey(t.Context(), b, category.Expense, "kafici"); !errors.Is(err, category.ErrNotFound) {
		t.Errorf("B's ByNameKey = %v, want ErrNotFound", err)
	}

	changed := anas
	changed.Name, changed.Archived = "Bars", true

	if err := repo.Update(t.Context(), b, changed); !errors.Is(err, category.ErrNotFound) {
		t.Errorf("B's Update = %v, want ErrNotFound", err)
	}

	if err := repo.Delete(t.Context(), b, anas.ID); !errors.Is(err, category.ErrNotFound) {
		t.Errorf("B's Delete = %v, want ErrNotFound", err)
	}

	if err := repo.SetOrder(t.Context(), b, []uuid.UUID{anas.ID}); err != nil {
		t.Errorf("B's SetOrder: %v", err)
	}

	if got, err := repo.Get(t.Context(), a, anas.ID); err != nil || got != anas {
		t.Errorf("A's category = %+v, %v; want it unchanged: %+v", got, err, anas)
	}
}

func TestTwoUsersMayEachHaveTheSameNameButOneUserNotTwiceInAKind(t *testing.T) {
	d := dbtest.Open(t)
	repo := category.NewPostgres(d.Queries())
	a, b := dbtest.NewUser(t, d.Queries()), dbtest.NewUser(t, d.Queries())
	create := func(user uuid.UUID, kind category.Kind, name string) error {
		return repo.Create(t.Context(), user, category.Category{ID: uuid.Must(uuid.NewV7()), Kind: kind, Name: name})
	}

	if err := create(a, category.Expense, "Kafići"); err != nil {
		t.Fatalf("A's Kafići: %v", err)
	}

	if err := create(b, category.Expense, "kafici"); err != nil {
		t.Errorf("B's kafici = %v, want it created", err)
	}

	if err := create(a, category.Income, "KAFICI"); err != nil {
		t.Errorf("A's income KAFICI = %v, want it created", err)
	}

	if err := create(a, category.Expense, "Ka-fi-ci"); !errors.Is(err, category.ErrNameTaken) {
		t.Errorf("A's second expense Ka-fi-ci = %v, want ErrNameTaken", err)
	}
}

func TestOneUserNeverReadsAnotherUsersCategoryIdempotencyKey(t *testing.T) {
	d := dbtest.Open(t)
	repo := category.NewPostgres(d.Queries())
	a, b := dbtest.NewUser(t, d.Queries()), dbtest.NewUser(t, d.Queries())
	now := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	var ids [2]uuid.UUID
	for i, user := range []uuid.UUID{a, b} {
		ids[i] = uuid.Must(uuid.NewV7())
		if err := repo.Create(t.Context(), user, category.Category{ID: ids[i], Kind: category.Expense, Name: "Rent"}); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	if claimed, err := repo.ClaimKey(t.Context(), a, "same-key", []byte("a"), ids[0], now); err != nil || !claimed {
		t.Fatalf("A's ClaimKey = %v, %v; want claimed", claimed, err)
	}

	if _, _, err := repo.Key(t.Context(), b, "same-key"); !errors.Is(err, category.ErrNoKey) {
		t.Errorf("B's Key = %v, want ErrNoKey", err)
	}

	if claimed, err := repo.ClaimKey(t.Context(), b, "same-key", []byte("b"), ids[1], now); err != nil || !claimed {
		t.Errorf("B's ClaimKey = %v, %v; want claimed: keys are per user", claimed, err)
	}

	if err := repo.DeleteExpiredKeys(t.Context(), b, now.Add(time.Hour)); err != nil {
		t.Fatalf("B's DeleteExpiredKeys: %v", err)
	}

	if hash, id, err := repo.Key(t.Context(), a, "same-key"); err != nil || string(hash) != "a" || id != ids[0] {
		t.Errorf("A's Key = %q, %v, %v; want A's key kept", hash, id, err)
	}
}

func TestCategoriesAreListedByKindThenPosition(t *testing.T) {
	d := dbtest.Open(t)
	repo := category.NewPostgres(d.Queries())
	user := dbtest.NewUser(t, d.Queries())

	second := category.Category{ID: uuid.Must(uuid.NewV7()), Kind: category.Expense, Name: "Rent", Position: 1}
	first := category.Category{ID: uuid.Must(uuid.NewV7()), Kind: category.Expense, Name: "Food", Position: 0}
	income := category.Category{ID: uuid.Must(uuid.NewV7()), Kind: category.Income, Name: "Salary", Position: 0}

	for _, c := range []category.Category{income, second, first} {
		if err := repo.Create(t.Context(), user, c); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	got, err := repo.List(t.Context(), user)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	want := []category.Category{first, second, income}
	if len(got) != len(want) {
		t.Fatalf("List = %+v, want %+v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("List[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
