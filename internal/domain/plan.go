package domain

import "time"

// PlanLength is the number of days in a child's plan.
const PlanLength = 7

// Plan day statuses.
const (
	PlanDayDone   = "done"   // the day's activity was done as part of the plan
	PlanDayToday  = "today"  // the next day to do, open now
	PlanDayLocked = "locked" // a later day, or the next day while a plan day was already done today
)

// PlanItem is a day of one child's plan: the child's goal it serves (nil when the child had no
// goals) and the activity picked for it. CompletedOn is the local day it was done, nil until then.
type PlanItem struct {
	Day         int
	GoalID      *string
	Activity    Activity
	CompletedOn *time.Time
}

// PlanDay is a PlanItem with its status.
type PlanDay struct {
	PlanItem
	Status string
}

// Plan is one of a child's 7-day plans. Number counts the child's plans from 1; StartedOn is the
// local day it was made.
type Plan struct {
	ID        string
	ChildID   string
	Number    int
	StartedOn time.Time
	Days      []PlanDay
	Completed bool
}

// PlanPick asks for one activity for a plan day. GoalID is the day's goal, ChildGoalIDs all the
// child's goals; ExcludeIDs are activities already in the plan. Seed varies the tie-break.
type PlanPick struct {
	ChildID      string
	Age          int
	GoalID       *string
	ChildGoalIDs []string
	ExcludeIDs   []string
	Seed         string
}

// PlanGoals returns the goal of each of the PlanLength days of the child's plan number n: the
// child's goals in turn, starting one goal further on with each plan so that no goal always gets
// the extra days. No goals gives nil for every day.
func PlanGoals(goalIDs []string, n int) []*string {
	out := make([]*string, PlanLength)
	if len(goalIDs) == 0 {
		return out
	}

	start := (max(n, 1) - 1) % len(goalIDs)

	for i := range out {
		id := goalIDs[(start+i)%len(goalIDs)]
		out[i] = &id
	}

	return out
}

// BuildPlanDays works out each day's status. Days are done in order: the first day not done is
// open (today) unless doneToday says a plan day of the child was already done today, and stays
// open however many days pass (a missed day pauses the plan). Later days are locked. The second
// result reports whether every day is done.
func BuildPlanDays(items []PlanItem, doneToday bool) ([]PlanDay, bool) {
	var (
		days = make([]PlanDay, 0, len(items))
		open = true
	)

	for _, it := range items {
		d := PlanDay{PlanItem: it, Status: PlanDayLocked}

		switch {
		case it.CompletedOn != nil:
			d.Status = PlanDayDone
		case open:
			if !doneToday {
				d.Status = PlanDayToday
			}

			open = false
		}

		days = append(days, d)
	}

	return days, open && len(days) > 0
}
