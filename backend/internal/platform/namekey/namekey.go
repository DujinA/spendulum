package namekey

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Of is the key that makes two names the same: NFD without combining marks, đ as d, lowercase, letters and digits only.
func Of(name string) string {
	var b strings.Builder

	for _, r := range norm.NFD.String(name) {
		switch {
		case r == 'đ' || r == 'Đ':
			b.WriteRune('d')
		case unicode.Is(unicode.Mn, r):
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		}
	}

	return b.String()
}
