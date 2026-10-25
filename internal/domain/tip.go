package domain

import "time"

// MaxTipTextLength caps each tip text, counted in characters.
const MaxTipTextLength = 500

// Tip is a one- or two-sentence advice for parents ("Kun maslahati"). It has one topic and an age
// range. Statuses are the article ones.
type Tip struct {
	ID        string
	TextUz    string
	TextRu    string
	TextEn    string
	TopicID   string
	MinAge    int
	MaxAge    int
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
	// Locked is set on app lists for a free-plan user: the tip was not their daily tip, so its
	// texts are left out.
	Locked bool
}

// TipUpdate is a partial update. A nil field is left unchanged.
type TipUpdate struct {
	TextUz  *string
	TextRu  *string
	TextEn  *string
	TopicID *string
	MinAge  *int
	MaxAge  *int
	Status  *string
}

// IsEmpty reports whether the update changes nothing.
func (u TipUpdate) IsEmpty() bool {
	return u.TextUz == nil && u.TextRu == nil && u.TextEn == nil && u.TopicID == nil &&
		u.MinAge == nil && u.MaxAge == nil && u.Status == nil
}

// TipFilter narrows a tip listing. Zero values mean "no filter".
type TipFilter struct {
	Age     int
	TopicID string
	// Search matches tips whose text in any language contains it, ignoring case.
	Search string
	Status string
	// LockFor, when set, marks the tips that were never this user's daily tip as Locked (free plan).
	LockFor string
	Limit   int
	Offset  int
}

// DailyTip is the tip a user gets for one local day.
type DailyTip struct {
	Day time.Time // a date: midnight UTC of the local day
	Tip Tip
}
