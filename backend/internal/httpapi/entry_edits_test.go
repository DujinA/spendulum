package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/DujinA/spendulum/backend/internal/category"
)

type editedEntryJSON struct {
	ID           string  `json:"id"`
	Kind         string  `json:"kind"`
	Amount       int64   `json:"amount"`
	Currency     string  `json:"currency"`
	CategoryID   string  `json:"categoryId"`
	CategoryName string  `json:"categoryName"`
	VendorName   *string `json:"vendorName"`
	EntryDate    string  `json:"entryDate"`
	Note         string  `json:"note"`
	CreatedAt    string  `json:"createdAt"`
}

func decodeEdited(t *testing.T, res response) editedEntryJSON {
	t.Helper()

	var e editedEntryJSON
	if err := json.Unmarshal(res.body, &e); err != nil {
		t.Fatalf("body: %v (%s)", err, res.body)
	}

	return e
}

func (a *app) editEntry(t *testing.T, o owner, id, body string) response {
	t.Helper()

	return a.do(t, request{method: http.MethodPut, path: "/api/entries/" + id, json: body, cookie: o.cookie})
}

func (a *app) deleteEntry(t *testing.T, o owner, id string) response {
	t.Helper()

	return a.do(t, request{method: http.MethodDelete, path: "/api/entries/" + id, cookie: o.cookie})
}

func (a *app) restoreEntry(t *testing.T, o owner, id string) response {
	t.Helper()

	return a.do(t, request{method: http.MethodPost, path: "/api/entries/" + id + "/restore", cookie: o.cookie})
}

func (a *app) recorded(t *testing.T, o owner, body string) editedEntryJSON {
	t.Helper()

	res := a.createEntry(t, o, uuid.NewString(), body)
	assertStatus(t, res, http.StatusCreated)

	return decodeEdited(t, res)
}

func changeJSON(categoryID uuid.UUID, amount int64, date, vendorName, note string) string {
	return fmt.Sprintf(`{"amount":%d,"categoryId":%q,"entryDate":%q,"vendorName":%q,"note":%q}`, amount, categoryID, date, vendorName, note)
}

func listedIDs(p pageJSON) []string {
	ids := make([]string, len(p.Entries))
	for i, e := range p.Entries {
		ids[i] = e.ID
	}

	return ids
}

func TestAnEditIsStoredWithoutAnIdempotencyKeyAndKeepsTheKindAndCurrency(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)
	eatingOut := a.addCategory(t, ana.id, category.Category{Kind: category.Expense, Name: "Eating out", Position: 2})
	created := a.recorded(t, ana, expenseJSON(ana.groceries, 178080, "2026-03-01", "weekly shop"))

	res := a.editEntry(t, ana, created.ID, changeJSON(eatingOut.ID, -5000, "2026-02-27", " Maxi ", "  returned  "))

	assertStatus(t, res, http.StatusOK)
	assertMatchesSpec(t, res)

	got := decodeEdited(t, res)
	vendor := got.VendorName
	got.VendorName = nil
	want := editedEntryJSON{
		ID: created.ID, Kind: "expense", Amount: -5000, Currency: "RSD", CategoryID: eatingOut.ID.String(),
		CategoryName: "Eating out", EntryDate: "2026-02-27", Note: "returned", CreatedAt: created.CreatedAt,
	}

	if got != want || vendor == nil || *vendor != "Maxi" {
		t.Errorf("edited = %+v with vendor %v, want %+v with vendor Maxi", got, vendor, want)
	}

	if one := decodeEdited(t, a.do(t, request{path: "/api/entries/" + created.ID, cookie: ana.cookie})); one.Amount != -5000 || one.Note != "returned" {
		t.Errorf("GET after the edit = %+v, want the change stored", one)
	}

	if list := a.listEntries(t, ana, ""); len(list.Entries) != 1 || list.Entries[0].Amount != -5000 {
		t.Errorf("list after the edit = %+v, want the one edited entry", list.Entries)
	}

	cleared := decodeEdited(t, a.editEntry(t, ana, created.ID, fmt.Sprintf(`{"amount":100,"categoryId":%q,"entryDate":"2026-02-27"}`, ana.groceries)))
	if cleared.VendorName != nil || cleared.Note != "" {
		t.Errorf("an edit without vendor and note = %+v, want neither", cleared)
	}
}

func TestAnEditThatBreaksARuleAnswers422ProblemJSONAndChangesNothing(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)
	expense := a.recorded(t, ana, expenseJSON(ana.groceries, 178080, "2026-03-01", "secret shop"))
	income := a.recorded(t, ana, fmt.Sprintf(`{"kind":"income","amount":500,"categoryId":%q,"entryDate":"2026-03-01"}`, ana.salary))

	for name, tc := range map[string]struct{ id, body string }{
		"zero expense":     {expense.ID, changeJSON(ana.groceries, 0, "2026-03-01", "", "")},
		"income category":  {expense.ID, changeJSON(ana.salary, 987654321, "2026-03-01", "", "")},
		"negative income":  {income.ID, changeJSON(ana.salary, -987654321, "2026-03-01", "", "")},
		"blank vendor":     {expense.ID, changeJSON(ana.groceries, 987654321, "2026-03-01", "?!", "")},
		"unknown category": {expense.ID, changeJSON(uuid.Must(uuid.NewV7()), 987654321, "2026-03-01", "", "")},
	} {
		t.Run(name, func(t *testing.T) {
			res := a.editEntry(t, ana, tc.id, tc.body)

			assertStatus(t, res, http.StatusUnprocessableEntity)
			assertMatchesSpec(t, res)
			assertProblem(t, res, http.StatusUnprocessableEntity)
		})
	}

	list := a.listEntries(t, ana, "")
	for _, e := range list.Entries {
		if e.Amount != 178080 && e.Amount != 500 {
			t.Errorf("a refused edit was stored: %+v", e)
		}
	}

	for _, secret := range []string{"987654321", "secret shop"} {
		if strings.Contains(a.logs.String(), secret) {
			t.Errorf("logs contain %q:\n%s", secret, a.logs.String())
		}
	}
}

func TestADeletedEntryLeavesTheListAndTheVendorSheetUntilUndoRestoresTheSameEntry(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)
	kept := a.recorded(t, ana, expenseJSON(ana.groceries, 100, "2026-03-01", ""))
	deleted := a.spend(t, ana, "Maxi", "2026-03-02")
	before := decodeEdited(t, a.do(t, request{path: "/api/entries/" + deleted.ID, cookie: ana.cookie}))

	res := a.deleteEntry(t, ana, deleted.ID)

	assertStatus(t, res, http.StatusNoContent)
	assertMatchesSpec(t, res)

	if ids := listedIDs(a.listEntries(t, ana, "")); len(ids) != 1 || ids[0] != kept.ID {
		t.Errorf("list after delete = %v, want only %s", ids, kept.ID)
	}

	if res := a.do(t, request{path: "/api/entries/" + deleted.ID, cookie: ana.cookie}); res.StatusCode != http.StatusNotFound {
		t.Errorf("GET of a deleted entry = %d, want 404", res.StatusCode)
	}

	if m := a.searchVendors(t, ana, "expense", "Maxi"); m.Match == nil || m.Match.UseCount != 0 {
		t.Errorf("Maxi after its only entry was deleted = %+v, want 0 uses", m.Match)
	}

	assertStatus(t, a.deleteEntry(t, ana, deleted.ID), http.StatusNoContent)

	edit := a.editEntry(t, ana, deleted.ID, changeJSON(ana.groceries, 100, "2026-03-02", "", ""))
	assertStatus(t, edit, http.StatusNotFound)
	assertProblem(t, edit, http.StatusNotFound)

	restored := a.restoreEntry(t, ana, deleted.ID)

	assertStatus(t, restored, http.StatusOK)
	assertMatchesSpec(t, restored)

	if got := decodeEdited(t, restored); got.ID != before.ID || got.Amount != before.Amount || got.CreatedAt != before.CreatedAt ||
		got.VendorName == nil || *got.VendorName != "Maxi" {
		t.Errorf("restored = %+v, want the deleted entry %+v", got, before)
	}

	if ids := listedIDs(a.listEntries(t, ana, "")); len(ids) != 2 || ids[0] != deleted.ID {
		t.Errorf("list after Undo = %v, want %s back first", ids, deleted.ID)
	}

	assertStatus(t, a.restoreEntry(t, ana, deleted.ID), http.StatusOK)
}

func TestUndoOfASavedEntryDeletesItAndARetryOfItsCreateAnswers422(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)
	body := expenseJSON(ana.groceries, 178080, "2026-03-01", "")
	created := decodeEdited(t, a.createEntry(t, ana, "save-key", body))

	assertStatus(t, a.deleteEntry(t, ana, created.ID), http.StatusNoContent)

	retry := a.createEntry(t, ana, "save-key", body)

	assertStatus(t, retry, http.StatusUnprocessableEntity)
	assertMatchesSpec(t, retry)
	assertProblem(t, retry, http.StatusUnprocessableEntity)

	if list := a.listEntries(t, ana, ""); len(list.Entries) != 0 {
		t.Errorf("list = %+v, want no entry after Undo and a retry", list.Entries)
	}
}

func TestAnotherUserEditingDeletingOrRestoringAnEntryGets404(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)
	bob := a.owner(t, "bob@example.test")
	anas := a.recorded(t, ana, expenseJSON(ana.groceries, 178080, "2026-03-01", ""))

	for name, res := range map[string]response{
		"edit":   a.editEntry(t, bob, anas.ID, changeJSON(bob.groceries, 1, "2026-03-01", "", "")),
		"delete": a.deleteEntry(t, bob, anas.ID),
	} {
		t.Run(name, func(t *testing.T) {
			assertStatus(t, res, http.StatusNotFound)
			assertMatchesSpec(t, res)
			assertProblem(t, res, http.StatusNotFound)
		})
	}

	if list := a.listEntries(t, ana, ""); len(list.Entries) != 1 || list.Entries[0].Amount != 178080 {
		t.Errorf("Ana's list = %+v, want her entry unchanged", list.Entries)
	}

	assertStatus(t, a.deleteEntry(t, ana, anas.ID), http.StatusNoContent)

	res := a.restoreEntry(t, bob, anas.ID)
	assertStatus(t, res, http.StatusNotFound)
	assertMatchesSpec(t, res)

	if list := a.listEntries(t, ana, ""); len(list.Entries) != 0 {
		t.Errorf("Ana's list = %+v, want her entry still deleted", list.Entries)
	}
}
