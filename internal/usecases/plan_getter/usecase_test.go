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

var (
	now   = time.Date(2026, 10, 11, 9, 0, 0, 0, domain.Location)
	today = domain.LocalDay(now)
)

type fakeTx struct{ inTx bool }

func (f *fakeTx) InTx(ctx context.Context, h func(context.Context) error) error {
	f.inTx = true
	defer func() { f.inTx = false }()

	return h(ctx)
}

type fakeChildren struct {
	goals []string
	err   error
}

func (f fakeChildren) GetForParent(_ context.Context, id, _ string) (domain.Child, error) {
	return domain.Child{ID: id, Age: 9, GoalIDs: f.goals}, f.err
}

// fakePlans serves a catalogue of n activities a0..a(n-1) and keeps one current plan.
type fakePlans struct {
	tx       *fakeTx
	n        int
	plan     domain.Plan
	items    []domain.PlanItem
	unusable map[int]bool
	done     []time.Time
	picks    []domain.PlanPick
	created  int
	locked   bool
}

func (f *fakePlans) LockChild(context.Context, string) error {
	if !f.tx.inTx {
		panic("LockChild outside the transaction")
	}

	f.locked = true

	return nil
}

func (f *fakePlans) Current(context.Context, string) (domain.Plan, []domain.PlanItem, []bool, error) {
	usable := make([]bool, len(f.items))
	for i := range usable {
		usable[i] = !f.unusable[i]
	}

	return f.plan, append([]domain.PlanItem(nil), f.items...), usable, nil
}

func (f *fakePlans) Create(_ context.Context, childID string, n int, on time.Time, items []domain.PlanItem) (string, error) {
	f.created++
	f.plan = domain.Plan{ID: "p" + strconv.Itoa(n), ChildID: childID, Number: n, StartedOn: on}
	f.items = append([]domain.PlanItem(nil), items...)
	f.unusable = nil

	return f.plan.ID, nil
}

func (f *fakePlans) Pick(_ context.Context, p domain.PlanPick) (domain.Activity, error) {
	f.picks = append(f.picks, p)

	excluded := map[string]bool{}
	for _, id := range p.ExcludeIDs {
		excluded[id] = true
	}

	for i := range f.n {
		if id := "a" + strconv.Itoa(i); !excluded[id] {
			return domain.Activity{ID: id}, nil
		}
	}

	return domain.Activity{}, errs.ErrNoRecommendation
}

func (f *fakePlans) SetActivity(_ context.Context, _ string, day int, activityID string) error {
	f.items[day-1].Activity = domain.Activity{ID: activityID}

	return nil
}

func (f *fakePlans) DoneDays(context.Context, string) ([]time.Time, int, time.Time, error) {
	return f.done, len(f.done), time.Time{}, nil
}

func newUseCase(f *fakePlans, goals ...string) *UseCase {
	tx := &fakeTx{}
	f.tx = tx

	uc := New(nil, tx, fakeChildren{goals: goals}, f)
	uc.now = func() time.Time { return now }

	return uc
}

func goalOf(it domain.PlanItem) string {
	if it.GoalID == nil {
		return "-"
	}

	return *it.GoalID
}

func TestExecute_FirstPlanRotatesGoals(t *testing.T) {
	f := &fakePlans{n: 10}

	p, err := newUseCase(f, "A", "B", "C").Execute(context.Background(), "u1", "c1")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !f.locked || f.created != 1 || p.Number != 1 || !p.StartedOn.Equal(today) || len(p.Days) != domain.PlanLength {
		t.Fatalf("unexpected plan: %+v", p)
	}

	goals, seen := "", map[string]bool{}
	for _, d := range p.Days {
		goals += goalOf(d.PlanItem)
		seen[d.Activity.ID] = true
	}

	if goals != "ABCABCA" || len(seen) != domain.PlanLength {
		t.Fatalf("goals %s, %d distinct activities", goals, len(seen))
	}

	if p.Days[0].Status != domain.PlanDayToday || p.Days[1].Status != domain.PlanDayLocked {
		t.Fatalf("statuses: %s %s", p.Days[0].Status, p.Days[1].Status)
	}

	// The age is clamped to the activity range and the child's goals are preferred.
	if pk := f.picks[0]; pk.Age != domain.MaxChildAge || len(pk.ChildGoalIDs) != 3 || *pk.GoalID != "A" {
		t.Fatalf("pick: %+v", pk)
	}
}

func TestExecute_KeepsPlanAndRepairs(t *testing.T) {
	f := &fakePlans{n: 10}
	uc := newUseCase(f, "A")

	if _, err := uc.Execute(context.Background(), "u1", "c1"); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	yesterday := today.AddDate(0, 0, -1)
	f.items[0].CompletedOn = &yesterday
	f.unusable = map[int]bool{0: true, 3: true}

	p, err := uc.Execute(context.Background(), "u1", "c1")
	if err != nil {
		t.Fatalf("Execute again: %v", err)
	}

	// No new plan; the done day keeps its activity, the undone unpublished one gets a new one.
	if f.created != 1 || p.Days[0].Activity.ID != "a0" || p.Days[3].Activity.ID != "a7" {
		t.Fatalf("created %d, days %s %s", f.created, p.Days[0].Activity.ID, p.Days[3].Activity.ID)
	}

	if p.Days[0].Status != domain.PlanDayDone || p.Days[1].Status != domain.PlanDayToday {
		t.Fatalf("statuses: %s %s", p.Days[0].Status, p.Days[1].Status)
	}
}

func TestExecute_DoneTodayLocksNextDay(t *testing.T) {
	f := &fakePlans{n: 10}
	uc := newUseCase(f)

	if _, err := uc.Execute(context.Background(), "u1", "c1"); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	f.items[0].CompletedOn = &today
	f.done = []time.Time{today}

	p, err := uc.Execute(context.Background(), "u1", "c1")
	if err != nil || p.Days[1].Status != domain.PlanDayLocked || goalOf(p.Days[1].PlanItem) != "-" {
		t.Fatalf("got %+v %v", p.Days[1], err)
	}
}

func TestNext(t *testing.T) {
	f := &fakePlans{n: 10}
	uc := newUseCase(f, "A", "B", "C")

	if _, err := uc.Execute(context.Background(), "u1", "c1"); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if _, err := uc.Next(context.Background(), "u1", "c1"); !errors.Is(err, errs.ErrPlanNotDone) || f.created != 1 {
		t.Fatalf("unfinished plan: %v, created %d", err, f.created)
	}

	for i := range f.items {
		on := today.AddDate(0, 0, -7+i)
		f.items[i].CompletedOn = &on
	}

	p, err := uc.Next(context.Background(), "u1", "c1")
	if err != nil || f.created != 2 || p.Number != 2 || goalOf(p.Days[0].PlanItem) != "B" || p.Days[0].Status != domain.PlanDayToday {
		t.Fatalf("next plan: %+v %v", p, err)
	}

	if f.picks[len(f.picks)-1].Seed != "2-7" {
		t.Fatalf("seed: %s", f.picks[len(f.picks)-1].Seed)
	}
}

func TestExecute_SmallCatalogueRepeats(t *testing.T) {
	f := &fakePlans{n: 2}

	p, err := newUseCase(f).Execute(context.Background(), "u1", "c1")
	if err != nil || len(p.Days) != domain.PlanLength || p.Days[2].Activity.ID != "a0" {
		t.Fatalf("got %+v %v", p, err)
	}
}

func TestExecute_Errors(t *testing.T) {
	if _, err := newUseCase(&fakePlans{}).Execute(context.Background(), "u1", "c1"); !errors.Is(err, errs.ErrNoRecommendation) {
		t.Fatalf("empty catalogue: %v", err)
	}

	uc := New(nil, &fakeTx{}, fakeChildren{err: errs.ErrChildNotFound}, &fakePlans{})
	if _, err := uc.Execute(context.Background(), "u1", "c1"); !errors.Is(err, errs.ErrChildNotFound) {
		t.Fatalf("other child: %v", err)
	}
}

func TestStart(t *testing.T) {
	f := &fakePlans{n: 10}
	if err := newUseCase(f, "A").Start(context.Background(), domain.Child{ID: "c1", Age: 3, GoalIDs: []string{"A"}}); err != nil || f.created != 1 {
		t.Fatalf("Start: %v, created %d", err, f.created)
	}

	// An empty catalogue does not stop the child from being created.
	if err := newUseCase(&fakePlans{}).Start(context.Background(), domain.Child{ID: "c1"}); err != nil {
		t.Fatalf("Start, no activities: %v", err)
	}
}
