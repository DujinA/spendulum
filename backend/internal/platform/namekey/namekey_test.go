package namekey_test

import (
	"testing"

	"github.com/DujinA/spendulum/backend/internal/platform/namekey"
)

func TestNamesThatDifferOnlyInCaseDiacriticsSpacesAndPunctuationShareAKey(t *testing.T) {
	for name, want := range map[string]string{
		"Kafići":               "kafici",
		"kafici ":              "kafici",
		"KAFICI!":              "kafici",
		"Đak":                  "dak",
		"đak":                  "dak",
		"Café-Bar No. 2":       "cafebarno2",
		"Ćevapi & Pljeskavica": "cevapipljeskavica",
		"Кафана":               "кафана",
		"!!!":                  "",
	} {
		if got := namekey.Of(name); got != want {
			t.Errorf("Of(%q) = %q, want %q", name, got, want)
		}
	}
}
