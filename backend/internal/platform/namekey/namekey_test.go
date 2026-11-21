package namekey_test

import (
	"testing"

	"github.com/DujinA/spendulum/backend/internal/platform/namekey"
)

func TestOfIgnoresCaseDiacriticsSpacesAndPunctuation(t *testing.T) {
	for name, want := range map[string]string{
		"Maxi":           "maxi",
		"maxi ":          "maxi",
		"MAXI":           "maxi",
		"Kafana Šljivik": "kafanasljivik",
		"Ćevabdžinica":   "cevabdzinica",
		"Đurđevak":       "durdevak",
		"Pekara-Zora!":   "pekarazora",
		"7 Eleven":       "7eleven",
		"  ":             "",
	} {
		if got := namekey.Of(name); got != want {
			t.Errorf("Of(%q) = %q, want %q", name, got, want)
		}
	}
}
