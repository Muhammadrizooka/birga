package domain

import (
	"math"
	"time"
	"unicode/utf8"
)

// Child genders, stored as text and checked by children_gender_chk.
const (
	GenderMale   = "male"
	GenderFemale = "female"
)

// MaxChildAgeMonths is the oldest age a parent can pick, 12 years.
const MaxChildAgeMonths = 144

// MaxChildGoals caps how many goals a child can have.
const MaxChildGoals = 20

// MaxChildNameLength caps a child's name, counted in characters.
const MaxChildNameLength = 100

// Child is a child profile. It belongs to one or more parents (users) through user_children.
type Child struct {
	ID        string
	Name      string
	AgeMonths int
	Gender    string
	PhotoID   *string
	// GoalIDs are the goals the parent picked for this child; the child's plans rotate through
	// them. Never nil when read from the database.
	GoalIDs   []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ChildUpdate is a partial update. A nil field is left unchanged; PhotoID pointing to "" clears the photo.
type ChildUpdate struct {
	Name      *string
	AgeMonths *int
	Gender    *string
	PhotoID   *string
	GoalIDs   *[]string // replaces the whole list; it then affects the next plan, not the current one
}

// IsEmpty reports whether the update changes nothing.
func (u ChildUpdate) IsEmpty() bool {
	return u.Name == nil && u.AgeMonths == nil && u.Gender == nil && u.PhotoID == nil && u.GoalIDs == nil
}

// MaxFreeChildren is how many children a user without Birga Pro may add.
const MaxFreeChildren = 1

// ChildFilter narrows a child listing. A non-empty ParentID lists only that user's children.
type ChildFilter struct {
	ParentID string
	Limit    int
	Offset   int
}

// IsKnownGender reports whether gender is one of the stored values.
func IsKnownGender(gender string) bool {
	return gender == GenderMale || gender == GenderFemale
}

// IsValidChildAgeMonths reports whether m is one of the ages the app offers: 1 to 11 months, then
// every half year from 1 to 7 years, then every whole year up to 12 years.
func IsValidChildAgeMonths(m int) bool {
	switch {
	case m >= 1 && m < 12:
		return true
	case m >= 12 && m <= 84:
		return m%6 == 0
	default:
		return m > 84 && m <= MaxChildAgeMonths && m%12 == 0
	}
}

// AgeYears is the child's age in whole years, for the 2 to 6 year ranges of the content.
func (c Child) AgeYears() int { return c.AgeMonths / 12 } //nolint:mnd // months in a year

// ChildAgeToDB turns an age in months into what children.age stores: years as a float (1 month
// is 1/12). Every API speaks months; only the database keeps years.
func ChildAgeToDB(months int) float64 { return float64(months) / 12 } //nolint:mnd // months in a year

// ChildAgeFromDB turns children.age (years as a float) back into months.
func ChildAgeFromDB(years float64) int { return int(math.Round(years * 12)) } //nolint:mnd // months in a year

// IsValidChildName reports whether name is non-empty and at most MaxChildNameLength characters.
func IsValidChildName(name string) bool {
	return name != "" && utf8.RuneCountInString(name) <= MaxChildNameLength
}
