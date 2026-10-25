package todaylist

import (
	"context"
	"errors"
	"testing"
	"time"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

const (
	parentID = "11111111-1111-1111-1111-111111111111"
	childID  = "22222222-2222-2222-2222-222222222222"
	goalID   = "33333333-3333-3333-3333-333333333333"
	planAct  = "44444444-4444-4444-4444-444444444444"
	otherAct = "55555555-5555-5555-5555-555555555555"
	draftAct = "66666666-6666-6666-6666-666666666666"
)

var now = time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)

type fakeChildren struct{}

func (fakeChildren) GetForParent(_ context.Context, id, userID string) (domain.Child, error) {
	if id != childID || userID != parentID {
		return domain.Child{}, errs.ErrChildNotFound
	}

	return domain.Child{ID: id, Age: 8}, nil
}

type fakePlans struct {
	days []domain.PlanDay
	err  error
}

func (f fakePlans) Execute(context.Context, string, string) (domain.Plan, error) {
	return domain.Plan{Days: f.days}, f.err
}

type fakeActivities struct{ similar domain.SimilarFilter }

func (fakeActivities) Get(_ context.Context, id string) (domain.Activity, error) {
	switch id {
	case otherAct:
		return domain.Activity{ID: id, IsPublished: true, GoalIDs: []string{goalID}, MinAge: 3, MaxAge: 5}, nil
	case draftAct:
		return domain.Activity{ID: id}, nil
	}

	return domain.Activity{}, errs.ErrActivityNotFound
}

func (f *fakeActivities) Similar(_ context.Context, fl domain.SimilarFilter) ([]domain.Activity, error) {
	f.similar = fl

	return nil, nil
}

type fakeExtras struct {
	list   []domain.DayExtra
	picked domain.ExtraPick
}

func (f *fakeExtras) ListDay(context.Context, string, time.Time) ([]domain.DayExtra, error) {
	return f.list, nil
}

func (f *fakeExtras) Add(_ context.Context, _ string, _ time.Time, id string) error {
	f.list = append(f.list, domain.DayExtra{Activity: domain.Activity{ID: id}})

	return nil
}

func (f *fakeExtras) Pick(_ context.Context, p domain.ExtraPick, _ time.Time) (domain.Activity, error) {
	f.picked = p

	return domain.Activity{ID: otherAct}, nil
}

type fakeGoals struct{}

func (fakeGoals) CountActive(_ context.Context, ids []string) (int, error) {
	if len(ids) == 1 && ids[0] == goalID {
		return 1, nil
	}

	return 0, nil
}

func newUC(plans fakePlans, extras *fakeExtras, acts *fakeActivities) *UseCase {
	uc := New(nil, fakeChildren{}, plans, acts, extras, fakeGoals{})
	uc.now = func() time.Time { return now }

	return uc
}

func TestExecute(t *testing.T) {
	today := domain.LocalDay(now)
	yesterday := today.AddDate(0, 0, -1)

	tests := []struct {
		name      string
		plans     fakePlans
		extras    []domain.DayExtra
		wantDay   int
		wantCount int
	}{
		{"open day", fakePlans{days: []domain.PlanDay{
			{PlanItem: domain.PlanItem{Day: 1, CompletedOn: &yesterday}, Status: domain.PlanDayDone},
			{PlanItem: domain.PlanItem{Day: 2}, Status: domain.PlanDayToday},
		}}, nil, 2, 0},
		{"done today", fakePlans{days: []domain.PlanDay{
			{PlanItem: domain.PlanItem{Day: 1, CompletedOn: &today}, Status: domain.PlanDayDone},
			{PlanItem: domain.PlanItem{Day: 2}, Status: domain.PlanDayLocked},
		}}, []domain.DayExtra{{Done: true}, {}}, 1, 2},
		{"plan finished earlier", fakePlans{days: []domain.PlanDay{
			{PlanItem: domain.PlanItem{Day: 7, CompletedOn: &yesterday}, Status: domain.PlanDayDone},
		}}, nil, 0, 0},
		{"no activity fits", fakePlans{err: errs.ErrNoRecommendation}, nil, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newUC(tt.plans, &fakeExtras{list: tt.extras}, &fakeActivities{}).Execute(context.Background(), parentID, childID)
			if err != nil {
				t.Fatal(err)
			}

			if day := 0; got.Plan != nil {
				day = got.Plan.Day
				if day != tt.wantDay {
					t.Errorf("plan day %d, want %d", day, tt.wantDay)
				}
			} else if tt.wantDay != 0 {
				t.Errorf("no plan, want day %d", tt.wantDay)
			}

			if got.DoneCount != tt.wantCount || !got.Day.Equal(today) {
				t.Errorf("done %d day %v", got.DoneCount, got.Day)
			}
		})
	}

	if _, err := newUC(fakePlans{}, &fakeExtras{}, &fakeActivities{}).Execute(context.Background(), "x", childID); !errors.Is(err, errs.ErrChildNotFound) {
		t.Errorf("other parent: %v", err)
	}
}

func TestAddExtra(t *testing.T) {
	plans := fakePlans{days: []domain.PlanDay{{PlanItem: domain.PlanItem{Day: 1, Activity: domain.Activity{ID: planAct}}, Status: domain.PlanDayToday}}}
	req := ExtraRequest{UserID: parentID, Pro: true, ChildID: childID}

	t.Run("free", func(t *testing.T) {
		r := req
		r.Pro, r.GoalID = false, goalID

		if _, err := newUC(plans, &fakeExtras{}, &fakeActivities{}).AddExtra(context.Background(), r); !errors.Is(err, errs.ErrNeedsPro) {
			t.Fatal(err)
		}
	})

	t.Run("goal picks around today's list", func(t *testing.T) {
		extras := &fakeExtras{list: []domain.DayExtra{{Activity: domain.Activity{ID: draftAct}}}}
		r := req
		r.GoalID = goalID

		got, err := newUC(plans, extras, &fakeActivities{}).AddExtra(context.Background(), r)
		if err != nil || got.Activity.ID != otherAct || len(extras.list) != 2 {
			t.Fatalf("%+v %v", got, err)
		}

		if p := extras.picked; p.Age != domain.MaxChildAge || len(p.ExcludeIDs) != 2 || p.ExcludeIDs[0] != planAct {
			t.Errorf("pick %+v", p)
		}
	})

	for name, tc := range map[string]struct {
		goal, activity string
		want           error
	}{
		"both":         {goalID, otherAct, errs.ErrValidation},
		"neither":      {"", "", errs.ErrValidation},
		"unknown goal": {"77777777-7777-7777-7777-777777777777", "", errs.ErrUnknownGoals},
		"unpublished":  {"", draftAct, errs.ErrActivityNotFound},
		"bad uuid":     {"", "nope", errs.ErrValidation},
	} {
		t.Run(name, func(t *testing.T) {
			r := req
			r.GoalID, r.ActivityID = tc.goal, tc.activity

			if _, err := newUC(plans, &fakeExtras{}, &fakeActivities{}).AddExtra(context.Background(), r); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}

	t.Run("repeat and limit", func(t *testing.T) {
		extras := &fakeExtras{}
		for i := range domain.MaxDayExtras {
			extras.list = append(extras.list, domain.DayExtra{Activity: domain.Activity{ID: string(rune('a' + i))}})
		}

		extras.list[0].Activity.ID = otherAct
		r := req
		r.ActivityID = otherAct

		if got, err := newUC(plans, extras, &fakeActivities{}).AddExtra(context.Background(), r); err != nil || got.Activity.ID != otherAct {
			t.Fatalf("repeat: %+v %v", got, err)
		}

		extras.list[0].Activity.ID = "z"
		if _, err := newUC(plans, extras, &fakeActivities{}).AddExtra(context.Background(), r); !errors.Is(err, errs.ErrTooManyExtras) {
			t.Fatalf("limit: %v", err)
		}
	})
}

func TestSimilar(t *testing.T) {
	acts := &fakeActivities{}
	uc := newUC(fakePlans{}, &fakeExtras{}, acts)
	ctx := context.Background()

	if _, err := uc.Similar(ctx, parentID, false, otherAct, "", 0); !errors.Is(err, errs.ErrNeedsPro) {
		t.Errorf("free: %v", err)
	}

	if _, err := uc.Similar(ctx, parentID, true, otherAct, "", 0); err != nil || acts.similar.Limit != DefaultSimilarLimit ||
		acts.similar.MinAge != 3 || acts.similar.MaxAge != 5 || acts.similar.ChildID != "" {
		t.Errorf("by activity: %+v %v", acts.similar, err)
	}

	if _, err := uc.Similar(ctx, parentID, true, otherAct, childID, 2); err != nil || acts.similar.ChildID != childID || acts.similar.Age != domain.MaxChildAge || acts.similar.Limit != 2 {
		t.Errorf("for child: %+v %v", acts.similar, err)
	}

	for _, tc := range []struct {
		activity, child string
		limit           int
		want            error
	}{
		{otherAct, "", 11, errs.ErrValidation},
		{draftAct, "", 0, errs.ErrActivityNotFound},
		{"nope", "", 0, errs.ErrActivityNotFound},
		{otherAct, "nope", 0, errs.ErrValidation},
		{otherAct, "77777777-7777-7777-7777-777777777777", 0, errs.ErrChildNotFound},
	} {
		if _, err := uc.Similar(ctx, parentID, true, tc.activity, tc.child, tc.limit); !errors.Is(err, tc.want) {
			t.Errorf("%+v: got %v", tc, err)
		}
	}
}
