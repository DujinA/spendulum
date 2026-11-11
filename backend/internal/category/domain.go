package category

import "github.com/google/uuid"

type Kind string

const (
	Expense Kind = "expense"
	Income  Kind = "income"
)

// Each kind starts with one Other category, made by the create-account operation (ADR 0009).
const OtherName = "Other"

type Category struct {
	ID       uuid.UUID
	Kind     Kind
	Name     string
	Position int
}
