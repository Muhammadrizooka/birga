package domain

import (
	"slices"
	"time"
)

// Milestone domains ("Nutq va muloqot", "His-tuyg‘ular va do‘stlik", "Fikrlash", "Harakat"). The
// app holds the labels.
const (
	MilestoneCommunication = "communication"
	MilestoneSocial        = "social"
	MilestoneThinking      = "thinking"
	MilestoneMovement      = "movement"
)

// MilestoneDomains lists the domains in the order the app shows them.
var MilestoneDomains = []string{MilestoneCommunication, MilestoneSocial, MilestoneThinking, MilestoneMovement}

// IsMilestoneDomain reports whether d is one of MilestoneDomains.
func IsMilestoneDomain(d string) bool { return slices.Contains(MilestoneDomains, d) }

// Milestone stage bounds, in months (milestones_age_chk).
const (
	MinMilestoneAgeMonths = 2
	MaxMilestoneAgeMonths = 72
)

// Text limits, counted in characters.
const (
	MaxMilestoneTextLength   = 300
	MaxMilestoneAdviceLength = 3000
	MaxMilestoneActivities   = 10
)

// Milestone is something most children can do by AgeMonths ("4 va undan ko‘p so‘zli gaplar
// tuzadi"), in one domain, with optional advice for parents. Statuses are the article ones.
type Milestone struct {
	ID        string
	Domain    string
	AgeMonths int
	SortOrder int
	TextUz    string
	TextRu    string
	TextEn    string
	// Advice: "Bu nimani anglatadi", "Qanday rivojlantirish" (one step per line) and "Qachon
	// mutaxassisga murojaat qilish kerak", plus the related activities. All optional.
	MeaningUz    string
	MeaningRu    string
	MeaningEn    string
	HowToUz      string
	HowToRu      string
	HowToEn      string
	SpecialistUz string
	SpecialistRu string
	SpecialistEn string
	ActivityIDs  []string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	// AdviceLocked is set when the advice was left out for a free-plan user although it exists.
	AdviceLocked bool
}

// HasAdvice reports whether the milestone has advice: any advice text or related activity, or
// advice that was left out (AdviceLocked).
func (m Milestone) HasAdvice() bool {
	if m.AdviceLocked {
		return true
	}

	for _, s := range []string{m.MeaningUz, m.MeaningRu, m.MeaningEn, m.HowToUz, m.HowToRu, m.HowToEn,
		m.SpecialistUz, m.SpecialistRu, m.SpecialistEn} {
		if s != "" {
			return true
		}
	}

	return len(m.ActivityIDs) > 0
}

// WithoutAdvice returns m with the advice left out, as free-plan users get it.
func (m Milestone) WithoutAdvice() Milestone {
	m.AdviceLocked = m.HasAdvice()
	m.MeaningUz, m.MeaningRu, m.MeaningEn = "", "", ""
	m.HowToUz, m.HowToRu, m.HowToEn = "", "", ""
	m.SpecialistUz, m.SpecialistRu, m.SpecialistEn = "", "", ""
	m.ActivityIDs = []string{}

	return m
}

// MilestoneUpdate is a partial update. A nil field is left unchanged; ActivityIDs, when not nil,
// replaces the list.
type MilestoneUpdate struct {
	Domain       *string
	AgeMonths    *int
	SortOrder    *int
	TextUz       *string
	TextRu       *string
	TextEn       *string
	MeaningUz    *string
	MeaningRu    *string
	MeaningEn    *string
	HowToUz      *string
	HowToRu      *string
	HowToEn      *string
	SpecialistUz *string
	SpecialistRu *string
	SpecialistEn *string
	ActivityIDs  []string
	Status       *string
}

// Texts returns pointers to the update's text fields: the three required texts first, then the
// nine optional advice texts.
func (u *MilestoneUpdate) Texts() (required, advice []**string) {
	return []**string{&u.TextUz, &u.TextRu, &u.TextEn},
		[]**string{&u.MeaningUz, &u.MeaningRu, &u.MeaningEn, &u.HowToUz, &u.HowToRu, &u.HowToEn,
			&u.SpecialistUz, &u.SpecialistRu, &u.SpecialistEn}
}

// IsEmpty reports whether the update changes nothing.
func (u MilestoneUpdate) IsEmpty() bool {
	required, advice := u.Texts()
	for _, s := range append(required, advice...) {
		if *s != nil {
			return false
		}
	}

	return u.Domain == nil && u.AgeMonths == nil && u.SortOrder == nil && u.ActivityIDs == nil && u.Status == nil
}

// MilestoneFilter narrows a milestone listing. Zero values mean "no filter".
type MilestoneFilter struct {
	Domain    string
	AgeMonths int
	Status    string
	// Search matches milestones whose text in any language contains it, ignoring case.
	Search string
	Limit  int
	Offset int
}

// ChildMilestone is a milestone with whether a parent ticked it for the child.
type ChildMilestone struct {
	Milestone
	Achieved   bool
	AchievedAt *time.Time
}

// ChildMilestones is a child's milestones of one stage, grouped by domain in MilestoneDomains
// order. Stages lists the stages that have published milestones.
type ChildMilestones struct {
	AgeMonths int
	Stages    []int
	Achieved  int
	Total     int
	Domains   map[string][]ChildMilestone
}

// MilestoneDetail is one milestone for the app with its related published activities.
type MilestoneDetail struct {
	ChildMilestone
	Activities []Activity
}

// MonthlyReport sums up a child's month ("Oylik hisobot", Figma 44).
type MonthlyReport struct {
	Month         time.Time // the first day of the month
	NewMilestones int
	Completions   int
}

// StageFor returns the highest of stages at or below ageMonths, or the lowest stage when all are
// higher; 0 when there are no stages. stages must be sorted.
func StageFor(stages []int, ageMonths int) int {
	if len(stages) == 0 {
		return 0
	}

	out := stages[0]
	for _, s := range stages {
		if s <= ageMonths {
			out = s
		}
	}

	return out
}
