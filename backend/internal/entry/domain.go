package entry

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/DujinA/spendulum/backend/internal/category"
	"github.com/DujinA/spendulum/backend/internal/platform/namekey"
)

const KeyLifetime = 24 * time.Hour

// A RuleError is a request that breaks a rule of the domain; its message is shown to the user.
type RuleError string

func (e RuleError) Error() string { return string(e) }

const (
	ErrUnknownKind       RuleError = "The kind must be expense or income."
	ErrIncomeNotPositive RuleError = "An income must be more than 0."
	ErrExpenseZero       RuleError = "An expense must not be 0."
	ErrCategoryRequired  RuleError = "A category is required."
	ErrUnknownCategory   RuleError = "The category does not exist or is of the other kind."
	ErrEntryDateRequired RuleError = "An entry date is required."
	ErrKeyReused         RuleError = "This Idempotency-Key was already used for a different entry."
	ErrKeyEntryDeleted   RuleError = "The entry this Idempotency-Key recorded was deleted."
	ErrVendorNameBlank   RuleError = "A vendor name needs at least one letter or digit."
)

var ErrNotFound = errors.New("entry: not found")

type Entry struct {
	ID           uuid.UUID
	Kind         category.Kind
	Amount       int64
	Currency     string
	CategoryID   uuid.UUID
	CategoryName string
	VendorID     uuid.UUID
	VendorName   string
	EntryDate    time.Time
	Note         string
	CreatedAt    time.Time
}

// NewEntry is what the user records; the currency is the user's primary currency, and an empty vendor name is no vendor.
type NewEntry struct {
	Kind       category.Kind
	Amount     int64
	CategoryID uuid.UUID
	VendorName string
	EntryDate  time.Time
	Note       string
}

func (n NewEntry) normalized() (NewEntry, error) {
	c, err := Change{Amount: n.Amount, CategoryID: n.CategoryID, VendorName: n.VendorName, EntryDate: n.EntryDate, Note: n.Note}.normalized(n.Kind)
	if err != nil {
		return NewEntry{}, err
	}

	n.VendorName, n.Note = c.VendorName, c.Note

	return n, nil
}

// Change is an edit of an entry: its kind and currency never change, and an empty vendor name is no vendor.
type Change struct {
	Amount     int64
	CategoryID uuid.UUID
	VendorName string
	EntryDate  time.Time
	Note       string
}

func (c Change) normalized(kind category.Kind) (Change, error) {
	c.VendorName = strings.TrimSpace(c.VendorName)
	c.Note = strings.TrimSpace(c.Note)

	switch {
	case kind != category.Expense && kind != category.Income:
		return Change{}, ErrUnknownKind
	case kind == category.Income && c.Amount <= 0:
		return Change{}, ErrIncomeNotPositive
	case kind == category.Expense && c.Amount == 0:
		return Change{}, ErrExpenseZero
	case c.Amount < -9007199254740991 || c.Amount > 9007199254740991:
		return Change{}, RuleError("The amount is outside the supported range.")
	case c.CategoryID == uuid.Nil:
		return Change{}, ErrCategoryRequired
	case c.EntryDate.IsZero():
		return Change{}, ErrEntryDateRequired
	case c.VendorName != "" && namekey.Of(c.VendorName) == "":
		return Change{}, ErrVendorNameBlank
	}

	return c, nil
}

// fingerprint tells a retry of a request from a different request under the same key.
func (n NewEntry) fingerprint() []byte {
	h := sha256.New()
	h.Write([]byte(n.Kind))
	h.Write([]byte{0})
	_ = binary.Write(h, binary.BigEndian, n.Amount)
	h.Write(n.CategoryID[:])
	h.Write([]byte(n.EntryDate.Format(time.DateOnly)))
	h.Write([]byte{0})
	h.Write([]byte(n.Note))

	// Appended only when set, so a request without a vendor keeps the hash keys stored before vendors existed.
	if n.VendorName != "" {
		h.Write([]byte{0})
		h.Write([]byte(n.VendorName))
		h.Write([]byte{0})
	}

	return h.Sum(nil)
}

// Cursor is the position after the last entry of a page, in list order.
type Cursor struct {
	EntryDate time.Time
	CreatedAt time.Time
	ID        uuid.UUID
}

type Page struct {
	Entries []Entry
	Next    *Cursor
}
