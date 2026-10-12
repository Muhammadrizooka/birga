package domain

import (
	"testing"
	"time"
)

func oct(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC) }

func items(ids ...string) []PlanItem {
	out := make([]PlanItem, 0, len(ids))
	for i, id := range ids {
		out = append(out, PlanItem{Theme: PlanTheme{Day: i + 1}, Activity: Activity{ID: id}})
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
	plan := items("a", "b", "c")

	cases := []struct {
		name        string
		today       time.Time
		completions map[string][]time.Time
		want        []string
		completed   bool
	}{
		{"fresh", oct(10), nil, []string{"today", "locked", "locked"}, false},
		{"first done today", oct(10), map[string][]time.Time{"a": {oct(10)}}, []string{"done", "locked", "locked"}, false},
		{"first done yesterday", oct(11), map[string][]time.Time{"a": {oct(10)}}, []string{"done", "today", "locked"}, false},
		// b was done on the same day as a, so it does not count for day 2 until it is done again later.
		{"two on one day", oct(11), map[string][]time.Time{"a": {oct(10)}, "b": {oct(10)}}, []string{"done", "today", "locked"}, false},
		{"out of order", oct(12), map[string][]time.Time{"c": {oct(10)}}, []string{"today", "locked", "locked"}, false},
		{"all done", oct(13), map[string][]time.Time{"a": {oct(10)}, "b": {oct(11)}, "c": {oct(13)}}, []string{"done", "done", "done"}, true},
		// Completions before the plan started do not count.
		{"before start", oct(10), map[string][]time.Time{"a": {oct(9)}}, []string{"today", "locked", "locked"}, false},
	}

	for _, tc := range cases {
		got, completed := BuildPlanDays(plan, oct(10), tc.today, tc.completions)

		if s := statuses(got); completed != tc.completed || len(s) != len(tc.want) {
			t.Fatalf("%s: got %v completed=%v", tc.name, s, completed)
		}

		for i, s := range statuses(got) {
			if s != tc.want[i] {
				t.Fatalf("%s: got %v, want %v", tc.name, statuses(got), tc.want)
			}
		}
	}
}

func TestBuildPlanDays_CompletedOn(t *testing.T) {
	got, _ := BuildPlanDays(items("a", "b"), oct(10), oct(12), map[string][]time.Time{"a": {oct(11), oct(10)}, "b": {oct(10), oct(12)}})

	if !got[0].CompletedOn.Equal(oct(10)) || !got[1].CompletedOn.Equal(oct(12)) {
		t.Fatalf("completed on: %v %v", got[0].CompletedOn, got[1].CompletedOn)
	}
}
