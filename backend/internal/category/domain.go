package category

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/DujinA/spendulum/backend/internal/platform/namekey"
)

type Kind string

const (
	Expense Kind = "expense"
	Income  Kind = "income"
)

// Each kind starts with one Other category, made by the create-account operation (ADR 0009).
const OtherName = "Other"

const KeyLifetime = 24 * time.Hour

// A RuleError is a request that breaks a rule of the domain; its message is shown to the user.
type RuleError string

func (e RuleError) Error() string { return string(e) }

const (
	ErrUnknownKind     RuleError = "The kind must be expense or income."
	ErrNameRequired    RuleError = "A name is required."
	ErrNameNoLetters   RuleError = "A name needs at least one letter or digit."
	ErrKeyReused       RuleError = "This Idempotency-Key was already used for a different category."
	ErrOrderIncomplete RuleError = "The order must list every category of this kind once."
)

func nameTaken(existing Category) RuleError {
	return RuleError(fmt.Sprintf("An %s category named “%s” already exists.", existing.Kind, existing.Name))
}

var (
	ErrNotFound = errors.New("category: not found")
	ErrInUse    = errors.New("category: has entries")
)

type Category struct {
	ID       uuid.UUID
	Kind     Kind
	Name     string
	Note     string
	Position int
	Archived bool
}

// NameKey is what makes a name unique within a kind.
func (c Category) NameKey() string { return namekey.Of(c.Name) }

type NewCategory struct {
	Kind Kind
	Name string
	Note string
}

func (n NewCategory) normalized() (NewCategory, error) {
	n.Name = strings.TrimSpace(n.Name)
	n.Note = strings.TrimSpace(n.Note)

	if n.Kind != Expense && n.Kind != Income {
		return NewCategory{}, ErrUnknownKind
	}

	return n, checkName(n.Name)
}

// fingerprint tells a retry of a request from a different request under the same key.
func (n NewCategory) fingerprint() []byte {
	h := sha256.New()
	for _, s := range []string{string(n.Kind), n.Name, n.Note} {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}

	return h.Sum(nil)
}

// A nil field stays as it is; there is no Kind, because a category's kind never changes.
type Change struct {
	Name     *string
	Note     *string
	Archived *bool
}

func (c Category) with(ch Change) (Category, error) {
	if ch.Name != nil {
		c.Name = strings.TrimSpace(*ch.Name)
		if err := checkName(c.Name); err != nil {
			return Category{}, err
		}
	}

	if ch.Note != nil {
		c.Note = strings.TrimSpace(*ch.Note)
	}

	if ch.Archived != nil {
		c.Archived = *ch.Archived
	}

	return c, nil
}

// checkOrder answers ErrNotFound for an id that is not one of the user's categories.
func checkOrder(all []Category, kind Kind, ids []uuid.UUID) error {
	if kind != Expense && kind != Income {
		return ErrUnknownKind
	}

	kinds := make(map[uuid.UUID]Kind, len(all))
	inKind := 0

	for _, c := range all {
		kinds[c.ID] = c.Kind
		if c.Kind == kind {
			inKind++
		}
	}

	seen := make(map[uuid.UUID]bool, len(ids))

	for _, id := range ids {
		k, ok := kinds[id]

		switch {
		case !ok:
			return ErrNotFound
		case k != kind || seen[id]:
			return ErrOrderIncomplete
		}

		seen[id] = true
	}

	if len(ids) != inKind {
		return ErrOrderIncomplete
	}

	return nil
}

func checkName(name string) error {
	switch {
	case name == "":
		return ErrNameRequired
	case namekey.Of(name) == "":
		return ErrNameNoLetters
	}

	return nil
}
