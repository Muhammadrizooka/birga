package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Topic tags tips, articles and videos with a subject that is not a goal ("Ovqat", "Uyqu"),
// named in Uzbek, Russian and English. Names follow the goal rules (IsValidGoalName).
type Topic struct {
	ID        string
	NameUz    string
	NameRu    string
	NameEn    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TopicUpdate is a partial update. A nil field is left unchanged.
type TopicUpdate struct {
	NameUz *string
	NameRu *string
	NameEn *string
}

// IsEmpty reports whether the update changes nothing.
func (u TopicUpdate) IsEmpty() bool {
	return u.NameUz == nil && u.NameRu == nil && u.NameEn == nil
}

// NormalizeTopicID trims an optional topic id in place and reports whether it is absent, ""
// (no topic) or a UUID.
func NormalizeTopicID(id *string) bool {
	if id == nil {
		return true
	}

	*id = strings.TrimSpace(*id)

	return *id == "" || uuid.Validate(*id) == nil
}
