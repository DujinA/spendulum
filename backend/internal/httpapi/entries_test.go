package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/DujinA/spendulum/backend/internal/category"
)

type entryJSON struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Amount       int64  `json:"amount"`
	Currency     string `json:"currency"`
	CategoryID   string `json:"categoryId"`
	CategoryName string `json:"categoryName"`
	EntryDate    string `json:"entryDate"`
	Note         string `json:"note"`
}

type pageJSON struct {
	Entries    []entryJSON `json:"entries"`
	NextCursor *string     `json:"nextCursor"`
}

// owner is a logged-in user with an expense category "Groceries" and the default Other categories.
type owner struct {
	cookie    *http.Cookie
	groceries uuid.UUID
	salary    uuid.UUID
}

func (a *app) owner(t *testing.T, email string) owner {
	t.Helper()

	id := a.accountFor(t, email)
	groceries := a.addCategory(t, id, category.Category{Kind: category.Expense, Name: "Groceries", Position: 1})
	salary := a.addCategory(t, id, category.Category{Kind: category.Income, Name: "Salary", Position: 1})

	return owner{cookie: a.loginAs(t, email), groceries: groceries.ID, salary: salary.ID}
}

func expenseJSON(categoryID uuid.UUID, amount int64, date, note string) string {
	return fmt.Sprintf(`{"kind":"expense","amount":%d,"categoryId":%q,"entryDate":%q,"note":%q}`, amount, categoryID, date, note)
}

func (a *app) createEntry(t *testing.T, o owner, key, body string) response {
	t.Helper()

	return a.do(t, request{method: http.MethodPost, path: "/api/entries", json: body, cookie: o.cookie, header: map[string]string{"Idempotency-Key": key}})
}

func (a *app) listEntries(t *testing.T, o owner, query string) pageJSON {
	t.Helper()

	res := a.do(t, request{path: "/api/entries" + query, cookie: o.cookie})
	assertStatus(t, res, http.StatusOK)
	assertMatchesSpec(t, res)

	var p pageJSON
	if err := json.Unmarshal(res.body, &p); err != nil {
		t.Fatalf("body: %v", err)
	}

	return p
}

func decodeEntry(t *testing.T, res response) entryJSON {
	t.Helper()

	var e entryJSON
	if err := json.Unmarshal(res.body, &e); err != nil {
		t.Fatalf("body: %v (%s)", err, res.body)
	}

	return e
}

func TestCreatedExpenseIsInThePrimaryCurrencyAndListedFirst(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)

	a.createEntry(t, ana, "older", expenseJSON(ana.groceries, 50000, "2026-02-28", ""))
	a.advance(time.Minute)

	res := a.createEntry(t, ana, "k1", expenseJSON(ana.groceries, 178080, "2026-03-01", "weekly shop"))

	assertStatus(t, res, http.StatusCreated)
	assertMatchesSpec(t, res)

	got := decodeEntry(t, res)
	want := entryJSON{
		ID: got.ID, Kind: "expense", Amount: 178080, Currency: "RSD", CategoryID: ana.groceries.String(),
		CategoryName: "Groceries", EntryDate: "2026-03-01", Note: "weekly shop",
	}

	if got != want {
		t.Errorf("created = %+v, want %+v", got, want)
	}

	list := a.listEntries(t, ana, "")
	if len(list.Entries) != 2 || list.Entries[0] != want {
		t.Errorf("list = %+v, want the new entry first", list.Entries)
	}

	if one := a.do(t, request{path: "/api/entries/" + got.ID, cookie: ana.cookie}); decodeEntry(t, one) != want {
		t.Errorf("GET /api/entries/%s = %s, want %+v", got.ID, one.body, want)
	}
}

func TestRefundIsStoredAsANegativeExpense(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)

	res := a.createEntry(t, ana, "k1", expenseJSON(ana.groceries, -199000, "2026-03-01", "returned cable"))

	assertStatus(t, res, http.StatusCreated)

	if got := decodeEntry(t, res); got.Amount != -199000 || got.Kind != "expense" {
		t.Errorf("refund = %+v, want an expense of -199000", got)
	}
}

func TestZeroAmountNegativeIncomeOrAnotherKindsCategoryAnswers422ProblemJSON(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)

	for name, body := range map[string]string{
		"zero expense":     expenseJSON(ana.groceries, 0, "2026-03-01", ""),
		"zero income":      fmt.Sprintf(`{"kind":"income","amount":0,"categoryId":%q,"entryDate":"2026-03-01"}`, ana.salary),
		"negative income":  fmt.Sprintf(`{"kind":"income","amount":-100,"categoryId":%q,"entryDate":"2026-03-01"}`, ana.salary),
		"income category":  expenseJSON(ana.salary, 100, "2026-03-01", ""),
		"no category":      `{"kind":"expense","amount":100,"entryDate":"2026-03-01"}`,
		"unknown category": expenseJSON(uuid.Must(uuid.NewV7()), 100, "2026-03-01", ""),
	} {
		t.Run(name, func(t *testing.T) {
			res := a.createEntry(t, ana, "key "+name, body)

			assertStatus(t, res, http.StatusUnprocessableEntity)
			assertMatchesSpec(t, res)
			assertProblem(t, res, http.StatusUnprocessableEntity)
		})
	}

	if list := a.listEntries(t, ana, ""); len(list.Entries) != 0 {
		t.Errorf("refused entries were stored: %+v", list.Entries)
	}
}

func TestNoteIsStoredTrimmedAndIsOptional(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)

	trimmed := decodeEntry(t, a.createEntry(t, ana, "k1", expenseJSON(ana.groceries, 100, "2026-03-01", "  coffee beans \n")))
	none := decodeEntry(t, a.createEntry(t, ana, "k2", fmt.Sprintf(`{"kind":"expense","amount":100,"categoryId":%q,"entryDate":"2026-03-01"}`, ana.groceries)))

	if trimmed.Note != "coffee beans" || none.Note != "" {
		t.Errorf("notes = %q and %q, want %q and empty", trimmed.Note, none.Note, "coffee beans")
	}
}

func TestRetryWithTheSameKeyAndBodyAnswersTheSameEntryAndCreatesNoOther(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)
	body := expenseJSON(ana.groceries, 178080, "2026-03-01", "")

	first := a.createEntry(t, ana, "retry-key", body)
	a.advance(time.Hour)
	retry := a.createEntry(t, ana, "retry-key", body)

	assertStatus(t, retry, http.StatusCreated)
	assertMatchesSpec(t, retry)

	if decodeEntry(t, retry) != decodeEntry(t, first) {
		t.Errorf("retry = %s, want the first entry %s", retry.body, first.body)
	}

	if list := a.listEntries(t, ana, ""); len(list.Entries) != 1 {
		t.Errorf("listed %d entries after a retry, want 1", len(list.Entries))
	}
}

func TestSameKeyWithADifferentBodyAnswers422(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)

	a.createEntry(t, ana, "reused", expenseJSON(ana.groceries, 178080, "2026-03-01", ""))
	res := a.createEntry(t, ana, "reused", expenseJSON(ana.groceries, 178000, "2026-03-01", ""))

	assertStatus(t, res, http.StatusUnprocessableEntity)
	assertMatchesSpec(t, res)
	assertProblem(t, res, http.StatusUnprocessableEntity)

	if list := a.listEntries(t, ana, ""); len(list.Entries) != 1 {
		t.Errorf("listed %d entries, want 1", len(list.Entries))
	}
}

func TestAKeyIsKeptFor24Hours(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)
	body := expenseJSON(ana.groceries, 100, "2026-03-01", "")

	first := decodeEntry(t, a.createEntry(t, ana, "daily", body))

	a.advance(24*time.Hour - time.Second)

	if kept := decodeEntry(t, a.createEntry(t, ana, "daily", body)); kept.ID != first.ID {
		t.Errorf("a retry 1 s before 24 hours created %s, want the first entry %s", kept.ID, first.ID)
	}

	a.advance(time.Second)

	if fresh := decodeEntry(t, a.createEntry(t, ana, "daily", body)); fresh.ID == first.ID {
		t.Error("the key still answered the first entry after 24 hours")
	}
}

func TestKeysAreScopedByUser(t *testing.T) {
	a := newApp(t)
	ana, bob := a.owner(t, email), a.owner(t, "bob@example.test")

	anas := decodeEntry(t, a.createEntry(t, ana, "shared-key", expenseJSON(ana.groceries, 100, "2026-03-01", "")))
	res := a.createEntry(t, bob, "shared-key", expenseJSON(bob.groceries, 100, "2026-03-01", ""))

	assertStatus(t, res, http.StatusCreated)

	if bobs := decodeEntry(t, res); bobs.ID == anas.ID || bobs.CategoryID != bob.groceries.String() {
		t.Errorf("Bob's entry = %+v, want his own, not Ana's %s", bobs, anas.ID)
	}
}

func TestCreateEntryWithoutAnIdempotencyKeyAnswers400(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)

	res := a.do(t, request{method: http.MethodPost, path: "/api/entries", json: expenseJSON(ana.groceries, 100, "2026-03-01", ""), cookie: ana.cookie})

	assertStatus(t, res, http.StatusBadRequest)
	assertProblem(t, res, http.StatusBadRequest)
}

func TestEntriesArePagedWithACursor(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)

	for i, date := range []string{"2026-03-01", "2026-02-27", "2026-02-28"} {
		a.advance(time.Minute)
		a.createEntry(t, ana, fmt.Sprint("k", i), expenseJSON(ana.groceries, int64(100+i), date, ""))
	}

	first := a.listEntries(t, ana, "?limit=2")
	if len(first.Entries) != 2 || first.NextCursor == nil {
		t.Fatalf("first page = %+v, want 2 entries and a next cursor", first)
	}

	second := a.listEntries(t, ana, "?limit=2&cursor="+url.QueryEscape(*first.NextCursor))
	if len(second.Entries) != 1 || second.NextCursor != nil {
		t.Fatalf("second page = %+v, want 1 entry and no next cursor", second)
	}

	got := []string{first.Entries[0].EntryDate, first.Entries[1].EntryDate, second.Entries[0].EntryDate}
	if strings.Join(got, " ") != "2026-03-01 2026-02-28 2026-02-27" {
		t.Errorf("pages hold %v, want newest entry date first", got)
	}
}

func TestBadCursorOrLimitAnswers400(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)

	for _, query := range []string{"?cursor=not-a-cursor", "?limit=0", "?limit=101"} {
		res := a.do(t, request{path: "/api/entries" + query, cookie: ana.cookie})

		assertStatus(t, res, http.StatusBadRequest)
		assertProblem(t, res, http.StatusBadRequest)
	}
}

func TestOneUserNeitherListsNorFetchesAnotherUsersEntries(t *testing.T) {
	a := newApp(t)
	ana, bob := a.owner(t, email), a.owner(t, "bob@example.test")
	anas := decodeEntry(t, a.createEntry(t, ana, "k1", expenseJSON(ana.groceries, 100, "2026-03-01", "")))

	if list := a.listEntries(t, bob, ""); len(list.Entries) != 0 {
		t.Errorf("Bob lists %+v, want none of Ana's entries", list.Entries)
	}

	res := a.do(t, request{path: "/api/entries/" + anas.ID, cookie: bob.cookie})

	assertStatus(t, res, http.StatusNotFound)
	assertMatchesSpec(t, res)
	assertProblem(t, res, http.StatusNotFound)
}

func TestFractionalOrOversizedAmountAnswers400WithoutTheNumberInLogsOrResponse(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)

	for name, amount := range map[string]string{
		"fractional": "98765432.5",
		"oversized":  "92233720368547758070",
	} {
		t.Run(name, func(t *testing.T) {
			body := fmt.Sprintf(`{"kind":"expense","amount":%s,"categoryId":%q,"entryDate":"2026-03-01"}`, amount, ana.groceries)
			res := a.createEntry(t, ana, "k-"+name, body)

			assertStatus(t, res, http.StatusBadRequest)
			assertMatchesSpec(t, res)
			assertProblem(t, res, http.StatusBadRequest)

			number := strings.TrimSuffix(amount, ".5")
			if strings.Contains(a.logs.String(), number) {
				t.Errorf("logs contain %s:\n%s", amount, a.logs.String())
			}

			if strings.Contains(string(res.body), number) {
				t.Errorf("response contains %s: %s", amount, res.body)
			}

			if !strings.Contains(string(res.body), "amount") {
				t.Errorf("response %s does not name the amount field", res.body)
			}
		})
	}
}

func TestCreatingAnEntryNeverLogsTheAmountOrNote(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)

	a.createEntry(t, ana, "k1", expenseJSON(ana.groceries, 987654321, "2026-03-01", "secret shop"))
	a.createEntry(t, ana, "k1", expenseJSON(ana.groceries, 987654321, "2026-03-01", "secret shop"))
	a.createEntry(t, ana, "k2", expenseJSON(ana.groceries, 0, "2026-03-01", "secret shop"))

	for _, secret := range []string{"987654321", "9876543", "secret shop"} {
		if strings.Contains(a.logs.String(), secret) {
			t.Errorf("logs contain %q:\n%s", secret, a.logs.String())
		}
	}
}
