package vendors

import (
	"cmp"
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/DujinA/spendulum/backend/internal/platform/namekey"
)

const (
	maxNearMatches        = 3
	nearMatchMinKeyLength = 4
	nearMatchMaxDistance  = 2
)

var (
	ErrNotFound  = errors.New("vendors: not found")
	ErrBlankName = errors.New("vendors: a name needs a letter or a digit")
)

// A Vendor as seen for one kind of entry: how many entries of that kind name it, and what the latest of them had.
type Vendor struct {
	ID       uuid.UUID
	Name     string
	Key      string
	Archived bool
	UseCount int
	Latest   *LatestUse
}

// LatestUse is the category and currency of a vendor's latest entry of one kind, by entry date, then creation time.
type LatestUse struct {
	CategoryID   uuid.UUID
	CategoryName string
	Currency     string
}

type Matches struct {
	Match   *Vendor
	Vendors []Vendor
	// Offered only when there is no match, so a typo does not make a second vendor.
	NearMatches []Vendor
}

// Search finds what the Vendor sheet offers for a typed name; an empty name lists every unarchived vendor.
func Search(all []Vendor, name string) Matches {
	key := namekey.Of(name)
	m := Matches{Vendors: []Vendor{}, NearMatches: []Vendor{}}

	var others []Vendor

	for _, v := range all {
		switch {
		case key != "" && v.Key == key:
			m.Match = &v
		case !v.Archived && strings.Contains(v.Key, key):
			m.Vendors = append(m.Vendors, v)
		default:
			others = append(others, v)
		}
	}

	slices.SortFunc(m.Vendors, mostUsedFirst)

	if m.Match != nil || utf8.RuneCountInString(key) < nearMatchMinKeyLength {
		return m
	}

	type near struct {
		v        Vendor
		distance int
	}

	var candidates []near

	for _, v := range others {
		if d := distance(key, v.Key); d <= nearMatchMaxDistance {
			candidates = append(candidates, near{v, d})
		}
	}

	slices.SortFunc(candidates, func(a, b near) int {
		return cmp.Or(cmp.Compare(a.distance, b.distance), mostUsedFirst(a.v, b.v))
	})

	for _, c := range candidates[:min(len(candidates), maxNearMatches)] {
		m.NearMatches = append(m.NearMatches, c.v)
	}

	return m
}

func mostUsedFirst(a, b Vendor) int {
	return cmp.Or(cmp.Compare(b.UseCount, a.UseCount), strings.Compare(a.Key, b.Key))
}

// distance is the Damerau-Levenshtein distance in its optimal string alignment form: an adjacent swap counts as 1.
func distance(a, b string) int {
	s, t := []rune(a), []rune(b)
	d := make([][]int, len(s)+1)

	for i := range d {
		d[i] = make([]int, len(t)+1)
		d[i][0] = i
	}

	for j := range d[0] {
		d[0][j] = j
	}

	for i := 1; i <= len(s); i++ {
		for j := 1; j <= len(t); j++ {
			cost := 1
			if s[i-1] == t[j-1] {
				cost = 0
			}

			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)

			if i > 1 && j > 1 && s[i-1] == t[j-2] && s[i-2] == t[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}

	return d[len(s)][len(t)]
}
