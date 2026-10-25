package domain

import "time"

// MaxDayExtras caps how many activities a parent may add to one child's day.
const MaxDayExtras = 10

// TodayPlan is the child's plan activity of the day: the open plan day, or the one done today.
type TodayPlan struct {
	Day      int
	Status   string // PlanDayToday or PlanDayDone
	Activity Activity
}

// DayExtra is an activity a Pro parent added to a child's day next to the plan activity. Done
// tells whether the child completed it that day.
type DayExtra struct {
	Activity Activity
	Done     bool
	AddedAt  time.Time
}

// Today is a child's list for one local day ("Bugungi reja"). Plan is nil when no plan day is open
// or done today.
type Today struct {
	Day       time.Time
	Plan      *TodayPlan
	Extras    []DayExtra
	DoneCount int
}

// ExtraPick asks for a new activity for a child's day serving GoalID, within Age and not in
// ExcludeIDs.
type ExtraPick struct {
	ChildID    string
	Age        int
	GoalID     string
	ExcludeIDs []string
}

// SimilarFilter asks for activities like ActivityID: sharing one of GoalIDs, not the activity
// itself and not in ExcludeIDs. With ChildID set they fit Age and the ones the child never did come first; otherwise
// they overlap MinAge..MaxAge.
type SimilarFilter struct {
	ActivityID string
	GoalIDs    []string
	ChildID    string
	Age        int
	MinAge     int
	MaxAge     int
	Limit      int
	ExcludeIDs []string
}
