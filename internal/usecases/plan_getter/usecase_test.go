package plangetter

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

var now = time.Date(2026, 10, 11, 9, 0, 0, 0, domain.Location)

type fakeTx struct{ inTx bool }

func (f *fakeTx) InTx(ctx context.Context, h func(context.Context) error) error {
	f.inTx = true
	defer func() { f.inTx = false }()

	return h(ctx)
}

type fakeChildren struct{ err error }

func (f fakeChildren) GetForParent(_ context.Context, id, _ string) (domain.Child, error) {
	return domain.Child{ID: id, Age: 9}, f.err
}

type fakeUsers struct{}

func (fakeUsers) Get(_ context.Context, id string) (domain.User, error) {
	return domain.User{ID: id, GoalIDs: []string{"g-user"}}, nil
}

// fakePlans serves a catalogue of n activities a0..a(n-1); unusable marks stored days whose
// activity was unpublished.
type fakePlans struct {
	tx        *fakeTx
	n         int
	items     []domain.PlanItem
	unusable  map[int]bool
	startedOn time.Time
	done      map[string][]time.Time
	picks     []domain.PlanPick
	locked    bool
	replaced  bool
}

func (f *fakePlans) Themes(context.Context) ([]domain.PlanTheme, error) {
	out := make([]domain.PlanTheme, 0, domain.PlanLength)
	for d := 1; d <= domain.PlanLength; d++ {
		out = append(out, domain.PlanTheme{Day: d})
	}

	return out, nil
}

func (f *fakePlans) LockChild(context.Context, string) error {
	if !f.tx.inTx {
		panic("LockChild outside the transaction")
	}

	f.locked = true

	return nil
}

func (f *fakePlans) Items(context.Context, string) ([]domain.PlanItem, []bool, time.Time, error) {
	usable := make([]bool, len(f.items))
	for i := range usable {
		usable[i] = !f.unusable[i]
	}

	return append([]domain.PlanItem(nil), f.items...), usable, f.startedOn, nil
}

func (f *fakePlans) Pick(_ context.Context, p domain.PlanPick) (domain.Activity, error) {
	f.picks = append(f.picks, p)

	for i := range f.n {
		id := "a" + strconv.Itoa(i)

		excluded := false
		for _, x := range p.ExcludeIDs {
			excluded = excluded || x == id
		}

		if !excluded {
			return domain.Activity{ID: id}, nil
		}
	}

	return domain.Activity{}, errs.ErrNoRecommendation
}

func (f *fakePlans) Replace(_ context.Context, _ string, startedOn time.Time, items []domain.PlanItem) error {
	f.items, f.startedOn, f.replaced = items, startedOn, true

	return nil
}

func (f *fakePlans) SetActivity(_ context.Context, _ string, day int, activityID string) error {
	f.items[day-1].Activity.ID = activityID

	return nil
}

func (f *fakePlans) CompletionDays(context.Context, string, time.Time) (map[string][]time.Time, error) {
	return f.done, nil
}

func newUseCase(plans *fakePlans) *UseCase {
	tx := &fakeTx{}
	plans.tx = tx
	uc := New(nil, tx, fakeChildren{}, fakeUsers{}, plans)
	uc.now = func() time.Time { return now }

	return uc
}

func TestExecute_BuildsPlan(t *testing.T) {
	plans := &fakePlans{n: 10}

	plan, err := newUseCase(plans).Execute(context.Background(), "u1", "c1", false)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !plans.locked || !plans.replaced || !plan.StartedOn.Equal(domain.LocalDay(now)) || len(plan.Days) != domain.PlanLength {
		t.Fatalf("unexpected plan: %+v", plan)
	}

	seen := map[string]bool{}
	for i, d := range plan.Days {
		if seen[d.Activity.ID] {
			t.Fatalf("activity %s repeated", d.Activity.ID)
		}

		seen[d.Activity.ID] = true

		want := domain.PlanDayLocked
		if i == 0 {
			want = domain.PlanDayToday
		}

		if d.Status != want {
			t.Fatalf("day %d status %s, want %s", i+1, d.Status, want)
		}
	}

	// The age is clamped and the parent's goals are preferred.
	if p := plans.picks[0]; p.Age != domain.MaxChildAge || len(p.PreferGoalIDs) != 1 || p.Day != 1 {
		t.Fatalf("unexpected pick: %+v", p)
	}
}

func TestExecute_SmallCatalogueRepeats(t *testing.T) {
	plans := &fakePlans{n: 3}

	plan, err := newUseCase(plans).Execute(context.Background(), "u1", "c1", false)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(plan.Days) != domain.PlanLength || plan.Days[3].Activity.ID != "a0" {
		t.Fatalf("expected repeats once the catalogue runs out: %+v", plan.Days)
	}
}

func TestExecute_EmptyCatalogue(t *testing.T) {
	if _, err := newUseCase(&fakePlans{}).Execute(context.Background(), "u1", "c1", false); !errors.Is(err, errs.ErrNoRecommendation) {
		t.Fatalf("expected no recommendation, got %v", err)
	}
}

func TestExecute_KeepsPlanAndRepairs(t *testing.T) {
	started := domain.LocalDay(now).AddDate(0, 0, -2)
	plans := &fakePlans{n: 10, startedOn: started, unusable: map[int]bool{0: true, 2: true}}

	for d := 1; d <= domain.PlanLength; d++ {
		plans.items = append(plans.items, domain.PlanItem{Theme: domain.PlanTheme{Day: d}, Activity: domain.Activity{ID: "a" + strconv.Itoa(d-1)}})
	}

	// Day 1 is done (so its unpublished activity stays); day 3 is not, so it gets a new one.
	plans.done = map[string][]time.Time{"a0": {started}}

	plan, err := newUseCase(plans).Execute(context.Background(), "u1", "c1", false)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if plans.replaced || !plan.StartedOn.Equal(started) {
		t.Fatalf("plan should be kept: %+v", plan)
	}

	if plan.Days[0].Activity.ID != "a0" || plan.Days[0].Status != domain.PlanDayDone || plan.Days[1].Status != domain.PlanDayToday {
		t.Fatalf("unexpected days: %+v", plan.Days[:2])
	}

	if plan.Days[2].Activity.ID != "a7" {
		t.Fatalf("day 3 should get a new activity, got %s", plan.Days[2].Activity.ID)
	}
}

func TestExecute_Restart(t *testing.T) {
	plans := &fakePlans{n: 10, startedOn: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		items: []domain.PlanItem{{Theme: domain.PlanTheme{Day: 1}, Activity: domain.Activity{ID: "a0"}}}}

	plan, err := newUseCase(plans).Execute(context.Background(), "u1", "c1", true)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !plans.replaced || !plan.StartedOn.Equal(domain.LocalDay(now)) || len(plan.Days) != domain.PlanLength {
		t.Fatalf("expected a new plan: %+v", plan)
	}
}

func TestExecute_NotParent(t *testing.T) {
	uc := newUseCase(&fakePlans{n: 10})
	uc.children = fakeChildren{err: errs.ErrChildNotFound}

	if _, err := uc.Execute(context.Background(), "u1", "c1", false); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}
