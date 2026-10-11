package domain

import "time"

// PlanLength is the number of days in a child's plan.
const PlanLength = 7

// Plan day statuses.
const (
	PlanDayDone   = "done"   // the day's activity was completed
	PlanDayToday  = "today"  // the next day to do, open now
	PlanDayLocked = "locked" // a later day, or the next day while the previous one was done today
)

// PlanTheme is one day of the 7-day programme: the same theme for every child, with an optional
// goal that the day's activity should serve.
type PlanTheme struct {
	Day     int
	TitleUz string
	TitleRu string
	Icon    string
	GoalID  *string
}

// PlanItem is a day of one child's plan: the theme and the activity picked for it.
type PlanItem struct {
	Theme    PlanTheme
	Activity Activity
}

// PlanDay is a PlanItem with the child's progress on it.
type PlanDay struct {
	PlanItem
	Status      string
	CompletedOn *time.Time // a date, as LocalDay returns it
}

// Plan is a child's 7-day plan. StartedOn is the local day it was made.
type Plan struct {
	ChildID   string
	StartedOn time.Time
	Days      []PlanDay
	Completed bool
}

// PlanPick asks for one activity for a plan day. ExcludeIDs are activities already in the plan;
// GoalID is the theme's goal, PreferGoalIDs the parent's goals.
type PlanPick struct {
	ChildID       string
	Age           int
	Day           int
	GoalID        *string
	PreferGoalIDs []string
	ExcludeIDs    []string
}

// BuildPlanDays works out each day's status. completions holds the local days on which the child
// did each activity since the plan started. Days are done in order, one per calendar day: day N
// counts as done by the first completion of its activity after the day N-1 was done. The first day
// not done is open (today) unless the previous day was done today; later days are locked.
func BuildPlanDays(items []PlanItem, startedOn, today time.Time, completions map[string][]time.Time) ([]PlanDay, bool) {
	var (
		days = make([]PlanDay, 0, len(items))
		prev = startedOn.AddDate(0, 0, -1)
		open = true
	)

	for _, it := range items {
		d := PlanDay{PlanItem: it, Status: PlanDayLocked}

		if open {
			if on, ok := firstAfter(completions[it.Activity.ID], prev); ok {
				d.Status, d.CompletedOn, prev = PlanDayDone, &on, on
			} else {
				if prev.Before(today) {
					d.Status = PlanDayToday
				}

				open = false
			}
		}

		days = append(days, d)
	}

	return days, open && len(days) > 0
}

// firstAfter returns the earliest day in days that is after t.
func firstAfter(days []time.Time, t time.Time) (time.Time, bool) {
	var (
		best  time.Time
		found bool
	)

	for _, d := range days {
		if d.After(t) && (!found || d.Before(best)) {
			best, found = d, true
		}
	}

	return best, found
}
