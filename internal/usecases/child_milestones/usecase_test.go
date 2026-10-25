package childmilestones

import (
	"context"
	"errors"
	"testing"
	"time"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

const (
	parentID  = "11111111-1111-1111-1111-111111111111"
	childID   = "22222222-2222-2222-2222-222222222222"
	published = "33333333-3333-3333-3333-333333333333"
	draft     = "44444444-4444-4444-4444-444444444444"
	actID     = "55555555-5555-5555-5555-555555555555"
)

type fakeChildren struct{ age int }

func (f fakeChildren) GetForParent(_ context.Context, id, userID string) (domain.Child, error) {
	if id != childID || userID != parentID {
		return domain.Child{}, errs.ErrChildNotFound
	}

	return domain.Child{ID: id, AgeMonths: f.age}, nil
}

type fakeMilestones struct {
	stages    []int
	askedAge  int
	askedFor  string
	marked    bool
	countFrom time.Time
	countTo   time.Time
}

func (f *fakeMilestones) Stages(context.Context) ([]int, error) { return f.stages, nil }

func (f *fakeMilestones) ForChild(_ context.Context, _ string, ageMonths int) ([]domain.ChildMilestone, error) {
	f.askedAge = ageMonths

	return []domain.ChildMilestone{
		{Milestone: domain.Milestone{ID: published, Domain: domain.MilestoneSocial, MeaningUz: "m"}, Achieved: true},
		{Milestone: domain.Milestone{ID: draft, Domain: domain.MilestoneSocial}},
	}, nil
}

func (f *fakeMilestones) GetForChild(_ context.Context, id, childID string) (domain.ChildMilestone, error) {
	f.askedFor = childID

	switch id {
	case published:
		return domain.ChildMilestone{Milestone: domain.Milestone{ID: id, Status: domain.ArticleStatusPublished,
			HowToUz: "h", ActivityIDs: []string{actID}}}, nil
	case draft:
		return domain.ChildMilestone{Milestone: domain.Milestone{ID: id, Status: domain.ArticleStatusInReview}}, nil
	}

	return domain.ChildMilestone{}, errs.ErrMilestoneNotFound
}

func (f *fakeMilestones) Mark(context.Context, string, string, string) error {
	f.marked = true

	return nil
}

func (f *fakeMilestones) Unmark(context.Context, string, string) error { return nil }

func (f *fakeMilestones) CountAchieved(_ context.Context, _ string, from, to time.Time) (int, error) {
	f.countFrom, f.countTo = from, to

	return 3, nil
}

type fakeActivities struct{}

func (fakeActivities) ListByIDs(_ context.Context, ids []string, _ bool) ([]domain.Activity, error) {
	return []domain.Activity{{ID: ids[0]}}, nil
}

type fakeCompletions struct{ from, to time.Time }

func (f *fakeCompletions) CountBetween(_ context.Context, _ string, from, to time.Time) (int, error) {
	f.from, f.to = from, to

	return 26, nil
}

func newUC(age int, m *fakeMilestones, c *fakeCompletions) *UseCase {
	uc := New(nil, fakeChildren{age: age * 12}, m, fakeActivities{}, c)
	uc.now = func() time.Time { return time.Date(2026, 10, 31, 20, 0, 0, 0, time.UTC) } // 1 Nov in Tashkent

	return uc
}

func TestList(t *testing.T) {
	ctx := context.Background()
	m := &fakeMilestones{stages: []int{24, 30, 36, 48, 60}}

	got, err := newUC(4, m, &fakeCompletions{}).List(ctx, parentID, false, childID, 0)
	if err != nil || got.AgeMonths != 48 || m.askedAge != 48 || got.Total != 2 || got.Achieved != 1 {
		t.Fatalf("child stage: %+v %v", got, err)
	}

	if first := got.Domains[domain.MilestoneSocial][0]; first.MeaningUz != "" || !first.AdviceLocked || !first.HasAdvice() {
		t.Errorf("free advice: %+v", first)
	}

	if got, _ := newUC(4, m, &fakeCompletions{}).List(ctx, parentID, true, childID, 30); got.AgeMonths != 30 ||
		got.Domains[domain.MilestoneSocial][0].MeaningUz != "m" {
		t.Errorf("pro stage 30: %+v", got)
	}

	if got, _ := newUC(1, m, &fakeCompletions{}).List(ctx, parentID, true, childID, 0); got.AgeMonths != 24 {
		t.Errorf("young child gets the first stage: %d", got.AgeMonths)
	}

	if _, err := newUC(4, m, &fakeCompletions{}).List(ctx, parentID, true, childID, 99); !errors.Is(err, errs.ErrValidation) {
		t.Errorf("bad stage: %v", err)
	}

	if _, err := newUC(4, m, &fakeCompletions{}).List(ctx, "x", true, childID, 0); !errors.Is(err, errs.ErrChildNotFound) {
		t.Errorf("other parent: %v", err)
	}
}

func TestGetAndMark(t *testing.T) {
	ctx := context.Background()
	m := &fakeMilestones{}
	uc := newUC(4, m, &fakeCompletions{})

	d, err := uc.Get(ctx, parentID, true, published, childID)
	if err != nil || len(d.Activities) != 1 || d.HowToUz != "h" || m.askedFor != childID {
		t.Fatalf("pro get: %+v %v", d, err)
	}

	d, err = uc.Get(ctx, parentID, false, published, "")
	if err != nil || len(d.Activities) != 0 || d.HowToUz != "" || !d.AdviceLocked || m.askedFor != noChild {
		t.Fatalf("free get: %+v %v", d, err)
	}

	if _, err := uc.Get(ctx, parentID, true, draft, ""); !errors.Is(err, errs.ErrMilestoneNotFound) {
		t.Errorf("draft: %v", err)
	}

	if err := uc.Mark(ctx, parentID, childID, draft); !errors.Is(err, errs.ErrMilestoneNotFound) || m.marked {
		t.Errorf("mark draft: %v", err)
	}

	if err := uc.Mark(ctx, parentID, childID, published); err != nil || !m.marked {
		t.Errorf("mark: %v", err)
	}
}

func TestReport(t *testing.T) {
	ctx := context.Background()
	m, c := &fakeMilestones{}, &fakeCompletions{}
	uc := newUC(4, m, c)

	if _, err := uc.Report(ctx, parentID, false, childID, ""); !errors.Is(err, errs.ErrNeedsPro) {
		t.Fatalf("free: %v", err)
	}

	r, err := uc.Report(ctx, parentID, true, childID, "")
	if err != nil || r.Month.Month() != time.November || r.NewMilestones != 3 || r.Completions != 26 {
		t.Fatalf("this month: %+v %v", r, err)
	}

	if _, err := uc.Report(ctx, parentID, true, childID, "2026-02"); err != nil {
		t.Fatal(err)
	}

	if !c.from.Equal(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)) || !c.to.Equal(time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("completion days %v..%v", c.from, c.to)
	}

	if !m.countFrom.Equal(time.Date(2026, 1, 31, 19, 0, 0, 0, time.UTC)) || !m.countTo.Equal(time.Date(2026, 2, 28, 19, 0, 0, 0, time.UTC)) {
		t.Errorf("tick range %v..%v", m.countFrom, m.countTo)
	}

	if _, err := uc.Report(ctx, parentID, true, childID, "october"); !errors.Is(err, errs.ErrValidation) {
		t.Errorf("bad month: %v", err)
	}
}
