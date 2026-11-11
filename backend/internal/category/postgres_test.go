package category_test

import (
	"testing"

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
