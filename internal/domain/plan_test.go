package domain

import (
	"testing"
	"time"
)

func oct(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC) }

func items(done ...int) []PlanItem {
	out := make([]PlanItem, 0, 3)
	for i := range 3 {
		it := PlanItem{Day: i + 1, Activity: Activity{ID: string(rune('a' + i))}}
		if i < len(done) {
			on := oct(done[i])
			it.CompletedOn = &on
		}

		out = append(out, it)
	}

	return out
}

func statuses(days []PlanDay) []string {
	out := make([]string, 0, len(days))
	for _, d := range days {
		out = append(out, d.Status)
	}

	return out
}

func TestBuildPlanDays(t *testing.T) {
	cases := []struct {
		name      string
		items     []PlanItem
		doneToday bool
		want      []string
		completed bool
	}{
		{"fresh", items(), false, []string{"today", "locked", "locked"}, false},
		{"first done today", items(10), true, []string{"done", "locked", "locked"}, false},
		// However long ago day 1 was done, day 2 waits: a missed day pauses the plan.
		{"paused", items(10), false, []string{"done", "today", "locked"}, false},
		// A plan day of an earlier plan done today keeps day 1 of a new plan locked until tomorrow.
		{"new plan, done today", items(), true, []string{"locked", "locked", "locked"}, false},
		{"all done", items(10, 11, 13), false, []string{"done", "done", "done"}, true},
	}

	for _, tc := range cases {
		got, completed := BuildPlanDays(tc.items, tc.doneToday)

		s := statuses(got)
		if completed != tc.completed || len(s) != len(tc.want) {
			t.Fatalf("%s: got %v completed=%v", tc.name, s, completed)
		}

		for i := range s {
			if s[i] != tc.want[i] {
				t.Fatalf("%s: got %v, want %v", tc.name, s, tc.want)
			}
		}
	}
}

func TestPlanGoals(t *testing.T) {
	str := func(ps []*string) string {
		out := ""
		for _, p := range ps {
			if p == nil {
				out += "-"
			} else {
				out += *p
			}
		}

		return out
	}

	cases := []struct {
		goals []string
		n     int
		want  string
	}{
		{[]string{"A", "B", "C"}, 1, "ABCABCA"},
		{[]string{"A", "B", "C"}, 2, "BCABCAB"},
		{[]string{"A", "B", "C"}, 4, "ABCABCA"},
		{[]string{"A"}, 3, "AAAAAAA"},
		{nil, 1, "-------"},
	}

	for _, tc := range cases {
		if got := str(PlanGoals(tc.goals, tc.n)); got != tc.want {
			t.Fatalf("PlanGoals(%v, %d) = %s, want %s", tc.goals, tc.n, got, tc.want)
		}
	}
}

func TestStreakDays(t *testing.T) {
	got := StreakDays([]time.Time{oct(12), oct(10)}, oct(9), oct(8), oct(13), oct(13))

	want := []string{"missed", "done", "missed", "done", "today"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}

	for i, d := range got {
		if d.Status != want[i] || !d.Day.Equal(oct(9+i)) {
			t.Fatalf("day %d: got %v %s, want %v %s", i, d.Day, d.Status, oct(9+i), want[i])
		}
	}

	if got := StreakDays(nil, time.Time{}, oct(1), oct(13), oct(13)); len(got) != 0 {
		t.Fatalf("no plan yet: got %v", got)
	}
}
