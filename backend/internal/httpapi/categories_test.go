package httpapi_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/DujinA/spendulum/backend/internal/category"
)

type categoryJSON struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Note     string `json:"note"`
	Archived bool   `json:"archived"`
}

func (a *app) categories(t *testing.T, c *http.Cookie, query string) []categoryJSON {
	t.Helper()

	res := a.do(t, request{path: "/api/categories" + query, cookie: c})
	assertStatus(t, res, http.StatusOK)
	assertMatchesSpec(t, res)

	var got []categoryJSON
	if err := json.Unmarshal(res.body, &got); err != nil {
		t.Fatalf("body: %v", err)
	}

	return got
}

func names(cats []categoryJSON) []string {
	out := make([]string, len(cats))
	for i, c := range cats {
		out[i] = c.Kind + " " + c.Name
		if c.Archived {
			out[i] += " (archived)"
		}
	}

	return out
}

func assertNames(t *testing.T, got []categoryJSON, want ...string) {
	t.Helper()

	if g := names(got); !slices.Equal(g, want) {
		t.Errorf("categories = %q, want %q", g, want)
	}
}

func TestManagedListHasBothKindsInManualOrderWithArchivedOnesMarked(t *testing.T) {
	a := newApp(t)
	user := a.account(t)
	c := a.login(t)

	a.addCategory(t, user, category.Category{Kind: category.Expense, Name: "Old hobby", Position: 1, Archived: true})
	a.addCategory(t, user, category.Category{Kind: category.Expense, Name: "Groceries", Position: 2, Note: "Food and drugstore"})
	a.addCategory(t, user, category.Category{Kind: category.Income, Name: "Salary", Position: 1})

	got := a.categories(t, c, "?includeArchived=true")

	assertNames(t, got, "expense Other", "expense Old hobby (archived)", "expense Groceries", "income Other", "income Salary")

	if got[2].Note != "Food and drugstore" || got[0].Note != "" {
		t.Errorf("notes = %q and %q, want the note and none", got[2].Note, got[0].Note)
	}
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

func (a *app) createCategory(t *testing.T, c *http.Cookie, key, body string) response {
	t.Helper()

	return a.do(t, request{method: http.MethodPost, path: "/api/categories", json: body, cookie: c, header: map[string]string{"Idempotency-Key": key}})
}

func decodeCategory(t *testing.T, res response) categoryJSON {
	t.Helper()

	var c categoryJSON
	if err := json.Unmarshal(res.body, &c); err != nil {
		t.Fatalf("body: %v (%s)", err, res.body)
	}

	return c
}

func TestCreatedCategoryHasItsKindAndOptionalNoteAndGoesLastInItsKind(t *testing.T) {
	a := newApp(t)
	a.account(t)
	c := a.login(t)

	withNote := a.createCategory(t, c, "k1", `{"kind":"income","name":"  Freelance ","note":" Side projects  "}`)
	assertStatus(t, withNote, http.StatusCreated)
	assertMatchesSpec(t, withNote)

	noNote := a.createCategory(t, c, "k2", `{"kind":"expense","name":"Groceries"}`)
	assertStatus(t, noNote, http.StatusCreated)

	got := decodeCategory(t, withNote)
	if want := (categoryJSON{ID: got.ID, Kind: "income", Name: "Freelance", Note: "Side projects"}); got != want || got.ID == "" {
		t.Errorf("created = %+v, want %+v with an id", got, want)
	}

	if got := decodeCategory(t, noNote); got.Kind != "expense" || got.Note != "" {
		t.Errorf("created = %+v, want an expense category with no note", got)
	}

	assertNames(t, a.categories(t, c, ""), "expense Other", "expense Groceries", "income Other", "income Freelance")
}

func TestCreatingACategoryWithoutANameOrWithAnUnknownKindAnswers422(t *testing.T) {
	a := newApp(t)
	a.account(t)
	c := a.login(t)

	for name, body := range map[string]string{
		"blank name":       `{"kind":"expense","name":"   "}`,
		"punctuation only": `{"kind":"expense","name":"!?"}`,
		"unknown kind":     `{"kind":"transfer","name":"Rent"}`,
	} {
		t.Run(name, func(t *testing.T) {
			res := a.createCategory(t, c, "key "+name, body)

			assertStatus(t, res, http.StatusUnprocessableEntity)
			assertMatchesSpec(t, res)
			assertProblem(t, res, http.StatusUnprocessableEntity)
		})
	}

	assertNames(t, a.categories(t, c, "?includeArchived=true"), "expense Other", "income Other")
}

func problemDetail(t *testing.T, res response) string {
	t.Helper()

	var p struct {
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(res.body, &p); err != nil {
		t.Fatalf("body is not JSON: %v (%s)", err, res.body)
	}

	return p.Detail
}

func TestCreatingANameWithTheSameKeyInTheSameKindAnswers422NamingTheExistingCategory(t *testing.T) {
	a := newApp(t)
	user := a.account(t)
	c := a.login(t)
	a.addCategory(t, user, category.Category{Kind: category.Expense, Name: "Kafići", Position: 1})

	for _, name := range []string{"kafici ", "KAFICI!", "Ka fi ći"} {
		t.Run(name, func(t *testing.T) {
			res := a.createCategory(t, c, "key "+name, `{"kind":"expense","name":"`+name+`"}`)

			assertStatus(t, res, http.StatusUnprocessableEntity)
			assertMatchesSpec(t, res)
			assertProblem(t, res, http.StatusUnprocessableEntity)

			if got, want := problemDetail(t, res), "An expense category named “Kafići” already exists."; got != want {
				t.Errorf("detail = %q, want %q", got, want)
			}
		})
	}

	assertNames(t, a.categories(t, c, ""), "expense Other", "expense Kafići", "income Other")
}

func TestTheSameNameInTheOtherKindIsAllowed(t *testing.T) {
	a := newApp(t)
	user := a.account(t)
	c := a.login(t)
	a.addCategory(t, user, category.Category{Kind: category.Expense, Name: "Kafići", Position: 1})

	res := a.createCategory(t, c, "k1", `{"kind":"income","name":"kafici"}`)

	assertStatus(t, res, http.StatusCreated)
	assertNames(t, a.categories(t, c, ""), "expense Other", "expense Kafići", "income Other", "income kafici")
}

func TestRetryingACategoryCreateWithTheSameKeyAndBodyAnswersTheSameCategory(t *testing.T) {
	a := newApp(t)
	a.account(t)
	c := a.login(t)
	body := `{"kind":"expense","name":"Kafići","note":"Coffee out"}`

	first := a.createCategory(t, c, "retry-key", body)
	a.advance(time.Hour)
	retry := a.createCategory(t, c, "retry-key", body)

	assertStatus(t, retry, http.StatusCreated)
	assertMatchesSpec(t, retry)

	if decodeCategory(t, retry) != decodeCategory(t, first) {
		t.Errorf("retry = %s, want the first category %s", retry.body, first.body)
	}

	assertNames(t, a.categories(t, c, ""), "expense Other", "expense Kafići", "income Other")
}

func TestReusingACategoryKeyWithADifferentBodyAnswers422(t *testing.T) {
	a := newApp(t)
	a.account(t)
	c := a.login(t)

	a.createCategory(t, c, "reused", `{"kind":"expense","name":"Kafići"}`)
	res := a.createCategory(t, c, "reused", `{"kind":"expense","name":"Rent"}`)

	assertStatus(t, res, http.StatusUnprocessableEntity)
	assertMatchesSpec(t, res)
	assertNames(t, a.categories(t, c, ""), "expense Other", "expense Kafići", "income Other")
}

func TestACategoryKeyIsKeptFor24Hours(t *testing.T) {
	a := newApp(t)
	a.account(t)
	c := a.login(t)
	body := `{"kind":"expense","name":"Kafići"}`

	a.createCategory(t, c, "day-key", body)
	a.advance(24*time.Hour + time.Second)
	res := a.createCategory(t, c, "day-key", `{"kind":"expense","name":"Rent"}`)

	assertStatus(t, res, http.StatusCreated)
	assertNames(t, a.categories(t, c, ""), "expense Other", "expense Kafići", "expense Rent", "income Other")
}

func (a *app) updateCategory(t *testing.T, c *http.Cookie, id, body string) response {
	t.Helper()

	return a.do(t, request{method: http.MethodPatch, path: "/api/categories/" + id, json: body, cookie: c})
}

func TestRenamingToANameWithTheSameKeyInTheSameKindAnswers422NamingTheExistingCategory(t *testing.T) {
	a := newApp(t)
	user := a.account(t)
	c := a.login(t)
	a.addCategory(t, user, category.Category{Kind: category.Expense, Name: "Kafići", Position: 1})
	bars := a.addCategory(t, user, category.Category{Kind: category.Expense, Name: "Bars", Position: 2})

	res := a.updateCategory(t, c, bars.ID.String(), `{"name":"kafici "}`)

	assertStatus(t, res, http.StatusUnprocessableEntity)
	assertMatchesSpec(t, res)
	assertProblem(t, res, http.StatusUnprocessableEntity)

	if got, want := problemDetail(t, res), "An expense category named “Kafići” already exists."; got != want {
		t.Errorf("detail = %q, want %q", got, want)
	}

	assertNames(t, a.categories(t, c, ""), "expense Other", "expense Kafići", "expense Bars", "income Other")
}

func TestRenamingACategoryToAVariantOfItsOwnNameIsAllowed(t *testing.T) {
	a := newApp(t)
	user := a.account(t)
	c := a.login(t)
	cafes := a.addCategory(t, user, category.Category{Kind: category.Expense, Name: "kafici", Position: 1})

	res := a.updateCategory(t, c, cafes.ID.String(), `{"name":" Kafići "}`)

	assertStatus(t, res, http.StatusOK)
	assertMatchesSpec(t, res)

	if got := decodeCategory(t, res); got.Name != "Kafići" {
		t.Errorf("renamed = %+v, want the name Kafići", got)
	}
}

func TestUpdatingACategoryChangesOnlyTheFieldsSentAndNeverItsKind(t *testing.T) {
	a := newApp(t)
	user := a.account(t)
	c := a.login(t)
	groceries := a.addCategory(t, user, category.Category{Kind: category.Expense, Name: "Groceries", Note: "Food", Position: 1})
	id := groceries.ID.String()

	renamed := decodeCategory(t, a.updateCategory(t, c, id, `{"name":"Food shop","kind":"income"}`))
	noted := decodeCategory(t, a.updateCategory(t, c, id, `{"note":"  Supermarket and market  "}`))

	if want := (categoryJSON{ID: id, Kind: "expense", Name: "Food shop", Note: "Food"}); renamed != want {
		t.Errorf("after rename = %+v, want %+v", renamed, want)
	}

	if want := (categoryJSON{ID: id, Kind: "expense", Name: "Food shop", Note: "Supermarket and market"}); noted != want {
		t.Errorf("after note change = %+v, want %+v", noted, want)
	}
}

func TestRenamingACategoryToABlankNameAnswers422(t *testing.T) {
	a := newApp(t)
	user := a.account(t)
	c := a.login(t)
	groceries := a.addCategory(t, user, category.Category{Kind: category.Expense, Name: "Groceries", Position: 1})

	res := a.updateCategory(t, c, groceries.ID.String(), `{"name":"  "}`)

	assertStatus(t, res, http.StatusUnprocessableEntity)
	assertMatchesSpec(t, res)
	assertNames(t, a.categories(t, c, ""), "expense Other", "expense Groceries", "income Other")
}

func TestArchivedCategoryLeavesThePickerStaysOnItsEntriesAndCanBeUnarchived(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)
	entry := decodeEntry(t, a.createEntry(t, ana, "k1", expenseJSON(ana.groceries, 100, "2026-03-01", "")))

	archived := a.updateCategory(t, ana.cookie, ana.groceries.String(), `{"archived":true}`)

	assertStatus(t, archived, http.StatusOK)
	assertMatchesSpec(t, archived)
	assertNames(t, a.categories(t, ana.cookie, ""), "expense Other", "income Other", "income Salary")
	assertNames(t, a.categories(t, ana.cookie, "?includeArchived=true"),
		"expense Other", "expense Groceries (archived)", "income Other", "income Salary")

	if got := decodeEntry(t, a.do(t, request{path: "/api/entries/" + entry.ID, cookie: ana.cookie})); got.CategoryName != "Groceries" {
		t.Errorf("entry after archiving = %+v, want it still in Groceries", got)
	}

	assertStatus(t, a.updateCategory(t, ana.cookie, ana.groceries.String(), `{"archived":false}`), http.StatusOK)
	assertNames(t, a.categories(t, ana.cookie, ""), "expense Other", "expense Groceries", "income Other", "income Salary")
}

func TestUpdatingAnUnknownCategoryAnswers404(t *testing.T) {
	a := newApp(t)
	a.account(t)
	c := a.login(t)

	res := a.updateCategory(t, c, uuid.Must(uuid.NewV7()).String(), `{"name":"Rent"}`)

	assertStatus(t, res, http.StatusNotFound)
	assertMatchesSpec(t, res)
	assertProblem(t, res, http.StatusNotFound)
}

func (a *app) deleteCategory(t *testing.T, c *http.Cookie, id string) response {
	t.Helper()

	return a.do(t, request{method: http.MethodDelete, path: "/api/categories/" + id, cookie: c})
}

func TestDeletingACategoryWithNoEntriesRemovesIt(t *testing.T) {
	a := newApp(t)
	user := a.account(t)
	c := a.login(t)
	unused := a.addCategory(t, user, category.Category{Kind: category.Expense, Name: "Unused", Position: 1})

	res := a.deleteCategory(t, c, unused.ID.String())

	assertStatus(t, res, http.StatusNoContent)
	assertMatchesSpec(t, res)
	assertNames(t, a.categories(t, c, "?includeArchived=true"), "expense Other", "income Other")
}

func TestDeletingACategoryWithEntriesAnswers409OfferingArchiveAndKeepsIt(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)
	a.createEntry(t, ana, "k1", expenseJSON(ana.groceries, 100, "2026-03-01", ""))

	res := a.deleteCategory(t, ana.cookie, ana.groceries.String())

	assertStatus(t, res, http.StatusConflict)
	assertMatchesSpec(t, res)
	assertProblem(t, res, http.StatusConflict)

	if got, want := problemDetail(t, res), "This category has entries, so it cannot be deleted. Archive it instead."; got != want {
		t.Errorf("detail = %q, want %q", got, want)
	}

	assertNames(t, a.categories(t, ana.cookie, ""), "expense Other", "expense Groceries", "income Other", "income Salary")
}

func TestDeletingAnUnknownCategoryAnswers404(t *testing.T) {
	a := newApp(t)
	a.account(t)
	c := a.login(t)

	res := a.deleteCategory(t, c, uuid.Must(uuid.NewV7()).String())

	assertStatus(t, res, http.StatusNotFound)
	assertMatchesSpec(t, res)
	assertProblem(t, res, http.StatusNotFound)
}

func (a *app) reorderCategories(t *testing.T, c *http.Cookie, kind string, ids ...string) response {
	t.Helper()

	body, err := json.Marshal(map[string]any{"kind": kind, "ids": ids})
	if err != nil {
		t.Fatal(err)
	}

	return a.do(t, request{method: http.MethodPut, path: "/api/categories/order", json: string(body), cookie: c})
}

// byName gives the ids of the user's categories by kind and name.
func byName(cats []categoryJSON) map[string]string {
	ids := make(map[string]string, len(cats))
	for _, c := range cats {
		ids[c.Kind+" "+c.Name] = c.ID
	}

	return ids
}

func TestReorderingAKindSavesTheNewOrderAndThePickerUsesIt(t *testing.T) {
	a := newApp(t)
	user := a.account(t)
	c := a.login(t)
	groceries := a.addCategory(t, user, category.Category{Kind: category.Expense, Name: "Groceries", Position: 1})
	rent := a.addCategory(t, user, category.Category{Kind: category.Expense, Name: "Rent", Position: 2})
	hobby := a.addCategory(t, user, category.Category{Kind: category.Expense, Name: "Old hobby", Position: 3, Archived: true})
	other := byName(a.categories(t, c, ""))["expense Other"]

	res := a.reorderCategories(t, c, "expense", rent.ID.String(), other, hobby.ID.String(), groceries.ID.String())

	assertStatus(t, res, http.StatusNoContent)
	assertMatchesSpec(t, res)
	assertNames(t, a.categories(t, c, ""), "expense Rent", "expense Other", "expense Groceries", "income Other")
	assertNames(t, a.categories(t, c, "?includeArchived=true"),
		"expense Rent", "expense Other", "expense Old hobby (archived)", "expense Groceries", "income Other")
}

func TestAnOrderThatMissesRepeatsOrMixesKindsAnswers422AndChangesNothing(t *testing.T) {
	a := newApp(t)
	user := a.account(t)
	c := a.login(t)
	rent := a.addCategory(t, user, category.Category{Kind: category.Expense, Name: "Rent", Position: 1}).ID.String()
	ids := byName(a.categories(t, c, ""))
	other, otherIncome := ids["expense Other"], ids["income Other"]

	for name, order := range map[string][]string{
		"misses one":      {rent},
		"repeats one":     {rent, other, rent},
		"other kind's id": {rent, other, otherIncome},
		"empty":           {},
	} {
		t.Run(name, func(t *testing.T) {
			res := a.reorderCategories(t, c, "expense", order...)

			assertStatus(t, res, http.StatusUnprocessableEntity)
			assertMatchesSpec(t, res)
			assertProblem(t, res, http.StatusUnprocessableEntity)
		})
	}

	assertNames(t, a.categories(t, c, ""), "expense Other", "expense Rent", "income Other")
}

func TestAnOrderWithAnUnknownIdAnswers404(t *testing.T) {
	a := newApp(t)
	a.account(t)
	c := a.login(t)
	other := byName(a.categories(t, c, ""))["expense Other"]

	res := a.reorderCategories(t, c, "expense", uuid.Must(uuid.NewV7()).String(), other)

	assertStatus(t, res, http.StatusNotFound)
	assertMatchesSpec(t, res)
	assertProblem(t, res, http.StatusNotFound)
}

func TestEachKindsOtherCategoryCanBeRenamedReorderedAndArchivedLikeAnyOther(t *testing.T) {
	a := newApp(t)
	user := a.account(t)
	c := a.login(t)
	gifts := a.addCategory(t, user, category.Category{Kind: category.Income, Name: "Gifts", Position: 1}).ID.String()
	ids := byName(a.categories(t, c, ""))
	otherExpense, otherIncome := ids["expense Other"], ids["income Other"]

	assertStatus(t, a.updateCategory(t, c, otherExpense, `{"name":"Miscellaneous"}`), http.StatusOK)
	assertStatus(t, a.reorderCategories(t, c, "income", gifts, otherIncome), http.StatusNoContent)
	assertStatus(t, a.updateCategory(t, c, otherIncome, `{"archived":true}`), http.StatusOK)

	assertNames(t, a.categories(t, c, "?includeArchived=true"),
		"expense Miscellaneous", "income Gifts", "income Other (archived)")
}

func TestAnotherUserMayUseTheSameNameButNeverReadsChangesReordersOrDeletesTheFirstUsersCategories(t *testing.T) {
	a := newApp(t)
	ana := a.accountFor(t, email)
	anaCookie := a.loginAs(t, email)
	anas := a.addCategory(t, ana, category.Category{Kind: category.Expense, Name: "Kafići", Position: 1}).ID.String()
	a.accountFor(t, "bob@example.test")
	bob := a.loginAs(t, "bob@example.test")

	assertStatus(t, a.createCategory(t, bob, "k1", `{"kind":"expense","name":"Kafići"}`), http.StatusCreated)
	bobsOther := byName(a.categories(t, bob, ""))["expense Other"]
	bobsCafes := byName(a.categories(t, bob, ""))["expense Kafići"]

	for name, res := range map[string]response{
		"rename":  a.updateCategory(t, bob, anas, `{"name":"Bars"}`),
		"archive": a.updateCategory(t, bob, anas, `{"archived":true}`),
		"delete":  a.deleteCategory(t, bob, anas),
		"reorder": a.reorderCategories(t, bob, "expense", anas, bobsOther, bobsCafes),
	} {
		t.Run(name, func(t *testing.T) {
			assertStatus(t, res, http.StatusNotFound)
			assertMatchesSpec(t, res)
			assertProblem(t, res, http.StatusNotFound)
		})
	}

	assertNames(t, a.categories(t, bob, "?includeArchived=true"), "expense Other", "expense Kafići", "income Other")
	assertNames(t, a.categories(t, anaCookie, "?includeArchived=true"), "expense Other", "expense Kafići", "income Other")
}
