package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/DujinA/spendulum/backend/internal/category"
)

type categoryJSON struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}

func TestCategoriesListTheUnarchivedOnesExpenseFirstInManualOrder(t *testing.T) {
	a := newApp(t)
	user := a.account(t)
	c := a.login(t)

	a.addCategory(t, user, category.Category{Kind: category.Expense, Name: "Old hobby", Position: 1, Archived: true})
	a.addCategory(t, user, category.Category{Kind: category.Expense, Name: "Groceries", Position: 2})

	res := a.do(t, request{path: "/api/categories", cookie: c})

	assertStatus(t, res, http.StatusOK)
	assertMatchesSpec(t, res)

	var got []categoryJSON
	if err := json.Unmarshal(res.body, &got); err != nil {
		t.Fatalf("body: %v", err)
	}

	want := []categoryJSON{{Kind: "expense", Name: "Other"}, {Kind: "expense", Name: "Groceries"}, {Kind: "income", Name: "Other"}}
	if len(got) != len(want) {
		t.Fatalf("categories = %+v, want %+v", got, want)
	}

	for i := range want {
		if got[i].Kind != want[i].Kind || got[i].Name != want[i].Name || got[i].ID == "" {
			t.Errorf("categories[%d] = %+v, want %+v with an id", i, got[i], want[i])
		}
	}
}
