package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/DujinA/spendulum/backend/internal/category"
	"github.com/DujinA/spendulum/backend/internal/vendors"
)

type vendorJSON struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
	UseCount int    `json:"useCount"`
	Latest   *struct {
		CategoryID   string `json:"categoryId"`
		CategoryName string `json:"categoryName"`
		Currency     string `json:"currency"`
	} `json:"latest"`
}

type vendorMatchesJSON struct {
	Match       *vendorJSON  `json:"match"`
	Vendors     []vendorJSON `json:"vendors"`
	NearMatches []vendorJSON `json:"nearMatches"`
}

type entryWithVendorJSON struct {
	ID         string  `json:"id"`
	Currency   string  `json:"currency"`
	VendorID   *string `json:"vendorId"`
	VendorName *string `json:"vendorName"`
}

func (a *app) searchVendors(t *testing.T, o owner, kind, name string) vendorMatchesJSON {
	t.Helper()

	query := url.Values{"kind": {kind}}
	if name != "" {
		query.Set("name", name)
	}

	res := a.do(t, request{path: "/api/vendors?" + query.Encode(), cookie: o.cookie})
	assertStatus(t, res, http.StatusOK)
	assertMatchesSpec(t, res)

	var m vendorMatchesJSON
	if err := json.Unmarshal(res.body, &m); err != nil {
		t.Fatalf("body: %v", err)
	}

	return m
}

// spend records an expense in Groceries with the vendor name, and answers its vendor.
func (a *app) spend(t *testing.T, o owner, vendorName, date string) entryWithVendorJSON {
	t.Helper()

	a.advance(time.Minute)

	body := fmt.Sprintf(`{"kind":"expense","amount":100,"categoryId":%q,"entryDate":%q,"vendorName":%q}`, o.groceries, date, vendorName)
	res := a.createEntry(t, o, uuid.NewString(), body)
	assertStatus(t, res, http.StatusCreated)
	assertMatchesSpec(t, res)

	var e entryWithVendorJSON
	if err := json.Unmarshal(res.body, &e); err != nil {
		t.Fatalf("body: %v", err)
	}

	return e
}

func vendorNames(vs []vendorJSON) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.Name
	}

	return out
}

func TestVendorSheetRanksVendorsByUseOfTheKindWithTheirLatestCategoryAndCurrency(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)
	eatingOut := a.addCategory(t, ana.id, category.Category{Kind: category.Expense, Name: "Eating out", Position: 2})

	a.spend(t, ana, "Lidl", "2026-03-01")
	a.spend(t, ana, "Maxi", "2026-03-01")
	a.spend(t, ana, "Maxi", "2026-02-27")
	a.createEntry(t, ana, "salary", fmt.Sprintf(`{"kind":"income","amount":100,"categoryId":%q,"entryDate":"2026-03-01","vendorName":"Lidl"}`, ana.salary))
	a.createEntry(t, ana, "salary 2", fmt.Sprintf(`{"kind":"income","amount":100,"categoryId":%q,"entryDate":"2026-03-01","vendorName":"Lidl"}`, ana.salary))
	a.createEntry(t, ana, "lunch", fmt.Sprintf(`{"kind":"expense","amount":100,"categoryId":%q,"entryDate":"2026-02-28","vendorName":"Maxi"}`, eatingOut.ID))

	expenses := a.searchVendors(t, ana, "expense", "")
	if got := vendorNames(expenses.Vendors); !slices.Equal(got, []string{"Maxi", "Lidl"}) {
		t.Fatalf("expense vendors = %v, want Maxi then Lidl", got)
	}

	maxi := expenses.Vendors[0]
	if maxi.UseCount != 3 || maxi.Latest == nil || maxi.Latest.CategoryName != "Groceries" || maxi.Latest.Currency != "RSD" {
		t.Errorf("Maxi = %+v (latest %+v), want 3 uses, latest in Groceries in RSD", maxi, maxi.Latest)
	}

	if got := vendorNames(a.searchVendors(t, ana, "income", "").Vendors); !slices.Equal(got, []string{"Lidl", "Maxi"}) {
		t.Errorf("income vendors = %v, want Lidl then Maxi", got)
	}
}

func TestANameWithTheSameKeyMatchesTheVendorWithoutAPromptAndSavingUsesIt(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)
	maxi := a.spend(t, ana, "Maxi", "2026-03-01")

	for _, typed := range []string{"maxi ", "MAXI"} {
		got := a.searchVendors(t, ana, "expense", typed)

		if got.Match == nil || got.Match.Name != "Maxi" || len(got.NearMatches) != 0 {
			t.Errorf("search %q = %+v, want Maxi as the match and no near-matches", typed, got)
		}

		if e := a.spend(t, ana, typed, "2026-03-01"); e.VendorID == nil || *e.VendorID != *maxi.VendorID || *e.VendorName != "Maxi" {
			t.Errorf("entry for %q has vendor %v %v, want Maxi %v", typed, e.VendorID, e.VendorName, *maxi.VendorID)
		}
	}

	if all := a.searchVendors(t, ana, "expense", "").Vendors; len(all) != 1 || all[0].UseCount != 3 {
		t.Errorf("vendors = %+v, want only Maxi with 3 uses", all)
	}
}

func TestANewNameOffersNearMatchesAndCreatingItNeverBlocks(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)
	kafeterija := a.spend(t, ana, "Kafeterija", "2026-03-01")

	got := a.searchVendors(t, ana, "expense", "Kafetrija")
	if got.Match != nil || !slices.Equal(vendorNames(got.NearMatches), []string{"Kafeterija"}) {
		t.Errorf("search = %+v, want no match and Kafeterija as a near-match", got)
	}

	if e := a.spend(t, ana, "Kafetrija", "2026-03-01"); e.VendorID == nil || *e.VendorID == *kafeterija.VendorID || *e.VendorName != "Kafetrija" {
		t.Errorf("entry has vendor %v %v, want a new vendor Kafetrija", e.VendorID, e.VendorName)
	}
}

func TestAShortNameThatMatchesNothingGetsNoNearMatch(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)
	a.spend(t, ana, "Maxi", "2026-03-01")

	if got := a.searchVendors(t, ana, "expense", "Mxi"); got.Match != nil || len(got.NearMatches) != 0 {
		t.Errorf("search = %+v, want no match and no near-match", got)
	}
}

func TestANewVendorIsCreatedOnlyTogetherWithItsEntry(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)

	for name, body := range map[string]string{
		"zero amount":      fmt.Sprintf(`{"kind":"expense","amount":0,"categoryId":%q,"entryDate":"2026-03-01","vendorName":"Mesara"}`, ana.groceries),
		"unknown category": fmt.Sprintf(`{"kind":"expense","amount":100,"categoryId":%q,"entryDate":"2026-03-01","vendorName":"Mesara"}`, uuid.New()),
	} {
		assertStatus(t, a.createEntry(t, ana, name, body), http.StatusUnprocessableEntity)
	}

	if got := a.searchVendors(t, ana, "expense", "Mesara"); got.Match != nil || len(got.Vendors) != 0 {
		t.Errorf("refused entries created a vendor: %+v", got)
	}

	e := a.spend(t, ana, "  Pekara Zora ", "2026-03-01")

	got := a.searchVendors(t, ana, "expense", "")
	if len(got.Vendors) != 1 || got.Vendors[0].Name != "Pekara Zora" || got.Vendors[0].ID != *e.VendorID || got.Vendors[0].UseCount != 1 {
		t.Errorf("vendors = %+v, want only the new Pekara Zora %v with 1 use", got.Vendors, e.VendorID)
	}
}

func TestAnArchivedVendorIsMatchedOrNearMatchedAndUnarchivedWhenUsed(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)
	archive := func(e entryWithVendorJSON) {
		t.Helper()

		if err := vendors.NewPostgres(a.db.Queries()).Archive(t.Context(), ana.id, uuid.MustParse(*e.VendorID)); err != nil {
			t.Fatalf("Archive: %v", err)
		}
	}

	knjizara := a.spend(t, ana, "Stara Knjižara", "2026-03-01")
	kafeterija := a.spend(t, ana, "Kafeterija", "2026-03-01")
	archive(knjizara)
	archive(kafeterija)

	if got := a.searchVendors(t, ana, "expense", ""); len(got.Vendors) != 0 {
		t.Errorf("vendors = %v, want no archived ones", vendorNames(got.Vendors))
	}

	if got := a.searchVendors(t, ana, "expense", "stara knjizara"); got.Match == nil || !got.Match.Archived {
		t.Errorf("search by exact name = %+v, want the archived vendor as the match", got)
	}

	if got := a.searchVendors(t, ana, "expense", "Kafetrija"); len(got.NearMatches) != 1 || !got.NearMatches[0].Archived {
		t.Errorf("search = %+v, want the archived Kafeterija as a near-match", got)
	}

	typed := a.spend(t, ana, "stara knjizara", "2026-03-02")
	picked := a.spend(t, ana, "Kafeterija", "2026-03-02")

	if *typed.VendorID != *knjizara.VendorID || *picked.VendorID != *kafeterija.VendorID {
		t.Errorf("entries name %v and %v, want the archived vendors %v and %v", *typed.VendorID, *picked.VendorID, *knjizara.VendorID, *kafeterija.VendorID)
	}

	if got := vendorNames(a.searchVendors(t, ana, "expense", "").Vendors); !slices.Equal(got, []string{"Kafeterija", "Stara Knjižara"}) {
		t.Errorf("vendors = %v, want both unarchived", got)
	}
}

func TestAnEntryWithoutAVendorNameOrWithABlankOneHasNoVendor(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)

	for key, body := range map[string]string{
		"absent": expenseJSON(ana.groceries, 100, "2026-03-01", ""),
		"blank":  fmt.Sprintf(`{"kind":"expense","amount":100,"categoryId":%q,"entryDate":"2026-03-01","vendorName":"  "}`, ana.groceries),
	} {
		res := a.createEntry(t, ana, key, body)
		assertStatus(t, res, http.StatusCreated)
		assertMatchesSpec(t, res)

		var e entryWithVendorJSON
		if err := json.Unmarshal(res.body, &e); err != nil || e.VendorID != nil || e.VendorName != nil {
			t.Errorf("%s vendor name: entry %s, want no vendor", key, res.body)
		}
	}

	if got := a.searchVendors(t, ana, "expense", ""); len(got.Vendors) != 0 {
		t.Errorf("vendors = %+v, want none", got.Vendors)
	}
}

func TestAVendorNameWithoutALetterOrDigitAnswers422(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)

	res := a.createEntry(t, ana, "k", fmt.Sprintf(`{"kind":"expense","amount":100,"categoryId":%q,"entryDate":"2026-03-01","vendorName":"!?"}`, ana.groceries))

	assertStatus(t, res, http.StatusUnprocessableEntity)
	assertMatchesSpec(t, res)
	assertProblem(t, res, http.StatusUnprocessableEntity)
}

func TestTheEntryListShowsEachEntrysVendor(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)

	a.spend(t, ana, "Maxi", "2026-03-02")
	a.createEntry(t, ana, "none", expenseJSON(ana.groceries, 100, "2026-03-01", ""))

	res := a.do(t, request{path: "/api/entries", cookie: ana.cookie})
	assertStatus(t, res, http.StatusOK)
	assertMatchesSpec(t, res)

	var page struct {
		Entries []entryWithVendorJSON `json:"entries"`
	}
	if err := json.Unmarshal(res.body, &page); err != nil {
		t.Fatalf("body: %v", err)
	}

	if len(page.Entries) != 2 || page.Entries[0].VendorName == nil || *page.Entries[0].VendorName != "Maxi" || page.Entries[1].VendorName != nil {
		t.Errorf("entries = %s, want Maxi on the first and no vendor on the second", res.body)
	}
}

func TestAnEntryInAnotherCurrencyThanThePrimaryAnswers422(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)

	euro := a.createEntry(t, ana, "eur", fmt.Sprintf(`{"kind":"expense","amount":100,"currency":"EUR","categoryId":%q,"entryDate":"2026-03-01"}`, ana.groceries))
	assertStatus(t, euro, http.StatusUnprocessableEntity)
	assertProblem(t, euro, http.StatusUnprocessableEntity)

	dinar := a.createEntry(t, ana, "rsd", fmt.Sprintf(`{"kind":"expense","amount":100,"currency":"RSD","categoryId":%q,"entryDate":"2026-03-01"}`, ana.groceries))
	assertStatus(t, dinar, http.StatusCreated)
}

func TestVendorsOfAnUnknownKindAnswer400(t *testing.T) {
	a := newApp(t)
	ana := a.owner(t, email)

	for _, query := range []string{"?kind=transfer", ""} {
		res := a.do(t, request{path: "/api/vendors" + query, cookie: ana.cookie})

		assertStatus(t, res, http.StatusBadRequest)
		assertProblem(t, res, http.StatusBadRequest)
	}
}

func TestOneUsersVendorsNeverMatchNorSuggestForAnother(t *testing.T) {
	a := newApp(t)
	ana, bob := a.owner(t, email), a.owner(t, "bob@example.test")
	anas := a.spend(t, ana, "Kafeterija", "2026-03-01")

	for _, typed := range []string{"", "Kafetrija", "Kafeterija", "kafe"} {
		if got := a.searchVendors(t, bob, "expense", typed); got.Match != nil || len(got.Vendors) != 0 || len(got.NearMatches) != 0 {
			t.Errorf("Bob's search %q = %+v, want nothing of Ana's", typed, got)
		}
	}

	bobs := a.spend(t, bob, "Kafeterija", "2026-03-01")
	if *bobs.VendorID == *anas.VendorID {
		t.Error("Bob's entry names Ana's vendor")
	}

	if got := a.searchVendors(t, ana, "expense", ""); len(got.Vendors) != 1 || got.Vendors[0].UseCount != 1 {
		t.Errorf("Ana's vendors = %+v, want her Kafeterija with 1 use", got.Vendors)
	}
}
