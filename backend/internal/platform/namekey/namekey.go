package namekey

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Of is the key two names are compared by: NFD with combining marks dropped, đ as d, lower case, letters and digits only.
func Of(name string) string {
	var b strings.Builder

	for _, r := range norm.NFD.String(name) {
		switch {
		case r == 'đ' || r == 'Đ':
			b.WriteRune('d')
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		}
	}

	return b.String()
}
