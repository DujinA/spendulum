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
	ErrUnknownKind        RuleError = "The kind must be expense or income."
	ErrIncomeNotPositive  RuleError = "An income must be more than 0."
	ErrExpenseZero        RuleError = "An expense must not be 0."
	ErrCategoryRequired   RuleError = "A category is required."
	ErrUnknownCategory    RuleError = "The category does not exist or is of the other kind."
	ErrEntryDateRequired  RuleError = "An entry date is required."
	ErrKeyReused          RuleError = "This Idempotency-Key was already used for a different entry."
	ErrVendorNameBlank    RuleError = "A vendor name needs at least one letter or digit."
	ErrCurrencyNotPrimary RuleError = "Entries are recorded in the primary currency."
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

// NewEntry is what the user records; an empty currency is the primary one, and an empty vendor name is no vendor.
type NewEntry struct {
	Kind       category.Kind
	Amount     int64
	Currency   string
	CategoryID uuid.UUID
	VendorName string
	EntryDate  time.Time
	Note       string
}

func (n NewEntry) normalized(primaryCurrency string) (NewEntry, error) {
	n.VendorName = strings.TrimSpace(n.VendorName)
	n.Note = strings.TrimSpace(n.Note)

	if n.Currency == "" {
		n.Currency = primaryCurrency
	}

	switch {
	case n.Kind != category.Expense && n.Kind != category.Income:
		return NewEntry{}, ErrUnknownKind
	case n.Kind == category.Income && n.Amount <= 0:
		return NewEntry{}, ErrIncomeNotPositive
	case n.Kind == category.Expense && n.Amount == 0:
		return NewEntry{}, ErrExpenseZero
	case n.Amount < -9007199254740991 || n.Amount > 9007199254740991:
		return NewEntry{}, RuleError("The amount is outside the supported range.")
	case n.CategoryID == uuid.Nil:
		return NewEntry{}, ErrCategoryRequired
	case n.EntryDate.IsZero():
		return NewEntry{}, ErrEntryDateRequired
	case n.Currency != primaryCurrency:
		return NewEntry{}, ErrCurrencyNotPrimary
	case n.VendorName != "" && namekey.Of(n.VendorName) == "":
		return NewEntry{}, ErrVendorNameBlank
	}

	return n, nil
}

// fingerprint tells a retry of a request from a different request under the same key.
func (n NewEntry) fingerprint() []byte {
	h := sha256.New()
	h.Write([]byte(n.Kind))
	h.Write([]byte{0})
	_ = binary.Write(h, binary.BigEndian, n.Amount)
	h.Write([]byte(n.Currency))
	h.Write(n.CategoryID[:])
	h.Write([]byte(n.VendorName))
	h.Write([]byte{0})
	h.Write([]byte(n.EntryDate.Format(time.DateOnly)))
	h.Write([]byte{0})
	h.Write([]byte(n.Note))

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
