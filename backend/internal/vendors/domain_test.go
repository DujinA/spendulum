package vendors_test

import (
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/DujinA/spendulum/backend/internal/platform/namekey"
	"github.com/DujinA/spendulum/backend/internal/vendors"
)

func vendor(name string, useCount int) vendors.Vendor {
	return vendors.Vendor{ID: uuid.Must(uuid.NewV7()), Name: name, Key: namekey.Of(name), UseCount: useCount}
}

func archived(name string) vendors.Vendor {
	v := vendor(name, 0)
	v.Archived = true

	return v
}

func names(vs []vendors.Vendor) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.Name
	}

	return out
}

func matchName(m vendors.Matches) string {
	if m.Match == nil {
		return ""
	}

	return m.Match.Name
}

func TestWithoutANameEveryUnarchivedVendorIsListedMostUsedFirst(t *testing.T) {
	all := []vendors.Vendor{vendor("Lidl", 2), archived("Old bakery"), vendor("Maxi", 9), vendor("Apoteka", 0), vendor("Kafeterija", 2)}

	got := vendors.Search(all, "")

	if want := []string{"Maxi", "Kafeterija", "Lidl", "Apoteka"}; !slices.Equal(names(got.Vendors), want) {
		t.Errorf("Vendors = %v, want %v", names(got.Vendors), want)
	}

	if got.Match != nil || len(got.NearMatches) != 0 {
		t.Errorf("Search without a name = %+v, want no match and no near-matches", got)
	}
}

func TestANameWithTheSameKeyIsTheMatchAndOffersNoNearMatch(t *testing.T) {
	all := []vendors.Vendor{vendor("Maxi", 3), vendor("Maxim", 1), vendor("Mexi", 1)}

	for _, typed := range []string{"maxi ", "MAXI", "Maxi"} {
		got := vendors.Search(all, typed)

		if matchName(got) != "Maxi" {
			t.Errorf("Search(%q) match = %q, want Maxi", typed, matchName(got))
		}

		if len(got.NearMatches) != 0 {
			t.Errorf("Search(%q) near-matches = %v, want none", typed, names(got.NearMatches))
		}

		if want := []string{"Maxim"}; !slices.Equal(names(got.Vendors), want) {
			t.Errorf("Search(%q) vendors = %v, want the other ones that contain the key, %v", typed, names(got.Vendors), want)
		}
	}
}

func TestANameWithoutAMatchOffersVendorsWithinDistance2(t *testing.T) {
	all := []vendors.Vendor{vendor("Kafeterija", 4), vendor("Kafana", 7)}

	for typed, want := range map[string][]string{
		"Kafetrija":  {"Kafeterija"},
		"Kafeterjia": {"Kafeterija"},
		// Two adjacent swaps count as 2; as two edits each, they would be 4.
		"Kaefterjia": {"Kafeterija"},
		"Kfeterja":   {"Kafeterija"},
		"Kfetrja":    {},
		"Kafanaaaa":  {},
	} {
		got := vendors.Search(all, typed)

		if got.Match != nil {
			t.Errorf("Search(%q) match = %q, want none", typed, matchName(got))
		}

		if !slices.Equal(names(got.NearMatches), want) {
			t.Errorf("Search(%q) near-matches = %v, want %v", typed, names(got.NearMatches), want)
		}
	}
}

func TestNearMatchesAreAtMost3ClosestFirstThenMostUsed(t *testing.T) {
	all := []vendors.Vendor{
		vendor("Kafeterijas", 9),
		vendor("Kafeterija", 1),
		vendor("Kafetria", 5),
		vendor("Kafetrja", 2),
	}

	got := vendors.Search(all, "Kafetrija")

	if want := []string{"Kafetria", "Kafetrja", "Kafeterija"}; !slices.Equal(names(got.NearMatches), want) {
		t.Errorf("near-matches = %v, want %v", names(got.NearMatches), want)
	}
}

func TestAKeyShorterThan4CharactersGetsNoNearMatch(t *testing.T) {
	all := []vendors.Vendor{vendor("Maxi", 3), vendor("DM", 1)}

	for _, typed := range []string{"Mxi", "D-N", "max!"} {
		if got := vendors.Search(all, typed); len(got.NearMatches) != 0 {
			t.Errorf("Search(%q) near-matches = %v, want none", typed, names(got.NearMatches))
		}
	}
}

func TestAVendorThatContainsTheKeyIsListedAndNotRepeatedAsANearMatch(t *testing.T) {
	all := []vendors.Vendor{vendor("Pekara", 2), vendor("Pekarica", 5), vendor("Mesara", 1)}

	got := vendors.Search(all, "pekar")

	if want := []string{"Pekarica", "Pekara"}; !slices.Equal(names(got.Vendors), want) {
		t.Errorf("Vendors = %v, want %v", names(got.Vendors), want)
	}

	if len(got.NearMatches) != 0 {
		t.Errorf("near-matches = %v, want none", names(got.NearMatches))
	}
}

func TestAnArchivedVendorIsNeverListedButCanBeTheMatchOrANearMatch(t *testing.T) {
	all := []vendors.Vendor{archived("Stara Knjižara"), archived("Kafeterija"), vendor("Knjižara Nova", 1)}

	if got := vendors.Search(all, "stara knjizara"); matchName(got) != "Stara Knjižara" || len(got.Vendors) != 0 {
		t.Errorf("Search for the archived name = %+v, want it as the match and nothing listed", got)
	}

	if got := vendors.Search(all, "Kafetrija"); !slices.Equal(names(got.NearMatches), []string{"Kafeterija"}) {
		t.Errorf("near-matches = %v, want the archived Kafeterija", names(got.NearMatches))
	}

	if got := vendors.Search(all, "knji"); !slices.Equal(names(got.Vendors), []string{"Knjižara Nova"}) {
		t.Errorf("Vendors = %v, want only the unarchived Knjižara Nova", names(got.Vendors))
	}
}
