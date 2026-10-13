package domain

import "time"

// MaxCompletionNoteLength is the longest reflection note, in characters (activity_completions_note_len_chk).
const MaxCompletionNoteLength = 1000

// Location is the time zone that decides which calendar day a completion belongs to. Uzbekistan
// is UTC+5 all year (no daylight saving), so a fixed zone needs no tzdata.
var Location = time.FixedZone("UZT", int((5 * time.Hour).Seconds())) //nolint:mnd // UTC+5

// Completion records that a child did an activity on a day, with the caregiver's optional reflection.
type Completion struct {
	ID          string
	ChildID     string
	ActivityID  string
	UserID      *string
	CompletedOn time.Time // a date: midnight UTC of the local day
	Note        *string
	CreatedAt   time.Time
	// PlanDay is the day of the child's plan this completion did, nil when it was not the open
	// plan day (it then stays in the history but does not count toward the streak).
	PlanDay *int
}

// CompletionFilter pages through one child's completions.
type CompletionFilter struct {
	ChildID string
	Limit   int
	Offset  int
}

// Streak summarises a child's practice. Only plan days count: a day is a streak day when a day of
// the child's plan was done on it.
type Streak struct {
	Current         int        // consecutive days up to today, or up to yesterday if today has no plan day done yet
	Longest         int        // longest run of consecutive days ever
	CompletedToday  bool       // a plan day was done today
	ThisWeek        int        // days with a plan day done this week (Monday to today)
	Total           int        // plan days done, over all plans
	LastCompletedOn *time.Time // nil before the first plan day is done
}

// LocalDay returns the calendar day of t in Location, as midnight UTC of that date.
func LocalDay(t time.Time) time.Time {
	y, m, d := t.In(Location).Date()

	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// ComputeStreak builds a Streak from the distinct days on which a plan day was done, newest first,
// as returned by LocalDay. today is LocalDay(now); total is the number of plan days done.
func ComputeStreak(days []time.Time, today time.Time, total int) Streak {
	s := Streak{Total: total}
	if len(days) == 0 {
		return s
	}

	last := days[0]
	s.LastCompletedOn = &last
	s.CompletedToday = last.Equal(today)

	weekStart := today.AddDate(0, 0, -daysSinceMonday(today))

	run := 0

	for i, d := range days {
		if !d.Before(weekStart) && !d.After(today) {
			s.ThisWeek++
		}

		if i > 0 && days[i-1].AddDate(0, 0, -1).Equal(d) {
			run++
		} else {
			run = 1
		}

		s.Longest = max(s.Longest, run)

		// The current streak is the first run, as long as it reaches today or yesterday.
		if run == i+1 && !last.Before(today.AddDate(0, 0, -1)) {
			s.Current = run
		}
	}

	return s
}

// daysSinceMonday is 0 on Monday and 6 on Sunday.
func daysSinceMonday(t time.Time) int {
	if t.Weekday() == time.Sunday {
		return 6 //nolint:mnd // Sunday ends the week
	}

	return int(t.Weekday() - time.Monday)
}

// Streak calendar day statuses.
const (
	StreakDayDone   = "done"   // a plan day of the child was done that day
	StreakDayMissed = "missed" // a past day with no plan day done
	StreakDayToday  = "today"  // today, nothing done yet: not missed until the day is over
)

// MaxStreakDays caps how many days one streak calendar request can cover.
const MaxStreakDays = 366

// StreakDay is one day of a child's streak calendar.
type StreakDay struct {
	Day    time.Time
	Status string
}

// StreakDays lists the days from `from` to `to` (both inclusive, LocalDay dates), skipping days
// before firstStart, the day the child's first plan started, and days after today. done holds the
// days a plan day was done, in any order. A zero firstStart (no plan yet) gives no days.
func StreakDays(done []time.Time, firstStart, from, to, today time.Time) []StreakDay {
	if firstStart.IsZero() {
		return nil
	}

	if from.Before(firstStart) {
		from = firstStart
	}

	if to.After(today) {
		to = today
	}

	isDone := make(map[time.Time]bool, len(done))
	for _, d := range done {
		isDone[d] = true
	}

	var out []StreakDay

	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		s := StreakDayMissed

		switch {
		case isDone[d]:
			s = StreakDayDone
		case d.Equal(today):
			s = StreakDayToday
		}

		out = append(out, StreakDay{Day: d, Status: s})
	}

	return out
}
