package domain

import (
	"regexp"
	"time"
	"unicode/utf8"
)

// MaxUserNameLength caps the display name, counted in characters.
const MaxUserNameLength = 100

var (
	// usernameRegex: 3..32 lowercase latin letters, digits, '_' or '.'.
	usernameRegex = regexp.MustCompile(`^[a-z0-9_.]{3,32}$`)
	// phoneRegex: E.164 with a leading '+', at most 15 characters to fit users.phone_number VARCHAR(15).
	phoneRegex = regexp.MustCompile(`^\+[1-9][0-9]{6,13}$`)
	// reminderTimeRegex: a 24-hour local time, HH:MM.
	reminderTimeRegex = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
)

// Relationships a user can have to the children, the users.relationship enum.
const (
	RelationshipFather   = "father"
	RelationshipMother   = "mother"
	RelationshipEducator = "educator"
	RelationshipNanny    = "nanny"
)

// User is an app user (a parent or caregiver). Nil fields are NULL in the database.
type User struct {
	ID           string
	Name         *string
	Username     *string
	PhoneNumber  *string
	PhotoID      *string
	Relationship *string
	// AgreedOnConditions is the consent to the terms given at sign-up.
	AgreedOnConditions bool
	// ReminderEnabled and ReminderTime are the daily reminder setting; ReminderTime is HH:MM in
	// Uzbekistan time.
	ReminderEnabled bool
	ReminderTime    string
	// ProUntil is when Birga Pro given by date ends; nil means never given. See IsPro.
	ProUntil  *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UserUpdate is a partial update. A nil field is left unchanged; a non-nil
// field pointing to "" clears the column (where the column may be cleared).
type UserUpdate struct {
	Name        *string
	Username    *string
	PhoneNumber *string
	PhotoID     *string
	// The fields below are the user's own settings, changed only through PATCH /v1/me.
	// AgreedOnConditions may only be set to true.
	AgreedOnConditions *bool
	ReminderEnabled    *bool
	ReminderTime       *string
}

// IsEmpty reports whether the update changes nothing.
func (u UserUpdate) IsEmpty() bool {
	return u.Name == nil && u.Username == nil && u.PhoneNumber == nil && u.PhotoID == nil &&
		u.AgreedOnConditions == nil && u.ReminderEnabled == nil && u.ReminderTime == nil
}

// UserFilter narrows a user listing.
type UserFilter struct {
	Limit  int
	Offset int
}

// IsValidUsername reports whether username matches the allowed format.
func IsValidUsername(username string) bool {
	return usernameRegex.MatchString(username)
}

// IsValidPhoneNumber reports whether phone is an E.164 number.
func IsValidPhoneNumber(phone string) bool {
	return phoneRegex.MatchString(phone)
}

// IsKnownRelationship reports whether relationship is one of the users.relationship enum values.
func IsKnownRelationship(relationship string) bool {
	switch relationship {
	case RelationshipFather, RelationshipMother, RelationshipEducator, RelationshipNanny:
		return true
	default:
		return false
	}
}

// IsValidReminderTime reports whether t is a 24-hour HH:MM time.
func IsValidReminderTime(t string) bool {
	return reminderTimeRegex.MatchString(t)
}

// IsValidUserName reports whether name fits the length limit.
func IsValidUserName(name string) bool {
	return utf8.RuneCountInString(name) <= MaxUserNameLength
}
