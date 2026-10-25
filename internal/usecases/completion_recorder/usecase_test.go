package completionrecorder

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

type fakeChildren struct{ err error }

func (f fakeChildren) GetForParent(_ context.Context, id, _ string) (domain.Child, error) {
	return domain.Child{ID: id, Age: 4}, f.err
}

type fakeActivities struct {
	a   domain.Activity
	err error
}

func (f fakeActivities) Get(context.Context, string) (domain.Activity, error) { return f.a, f.err }

type fakeCompletions struct {
	got    domain.Completion
	called bool
}

func (f *fakeCompletions) Create(_ context.Context, c domain.Completion) (domain.Completion, bool, error) {
	f.got, f.called = c, true

	return c, true, nil
}

type fakeTx struct{}

func (fakeTx) InTx(ctx context.Context, h func(context.Context) error) error { return h(ctx) }

type fakePlans struct {
	day    int
	marked string
}

func (*fakePlans) LockChild(context.Context, string) error { return nil }

func (f *fakePlans) MarkDone(_ context.Context, _, activityID string, on time.Time) (int, error) {
	f.marked = activityID + "@" + on.Format(time.DateOnly)

	return f.day, nil
}

func newUC(children fakeChildren, activities fakeActivities, completions *fakeCompletions) *UseCase {
	return newUCWithPlans(children, activities, completions, &fakePlans{})
}

func newUCWithPlans(children fakeChildren, activities fakeActivities, completions *fakeCompletions, plans *fakePlans) *UseCase {
	uc := New(nil, fakeTx{}, children, activities, completions, plans)
	uc.now = func() time.Time { return time.Date(2026, 10, 3, 21, 0, 0, 0, time.UTC) } // Oct 4 in Tashkent

	return uc
}

func TestExecute_Success(t *testing.T) {
	comp := &fakeCompletions{}
	note := "  yaxshi o'tdi  "

	_, err := newUC(fakeChildren{}, fakeActivities{a: domain.Activity{IsPublished: true}}, comp).
		Execute(context.Background(), Request{UserID: "u", ChildID: "c", ActivityID: "a", Note: &note, Pro: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if comp.got.CompletedOn.Format(time.DateOnly) != "2026-10-04" || *comp.got.Note != "yaxshi o'tdi" || *comp.got.UserID != "u" {
		t.Fatalf("unexpected completion: %+v", comp.got)
	}

	blank := "   "
	if _, err := newUC(fakeChildren{}, fakeActivities{a: domain.Activity{IsPublished: true}}, comp).
		Execute(context.Background(), Request{ChildID: "c", ActivityID: "a", Note: &blank, Pro: true}); err != nil || comp.got.Note != nil {
		t.Fatalf("blank note should be dropped: %v %+v", err, comp.got.Note)
	}
}

func TestExecute_Rejected(t *testing.T) {
	long := strings.Repeat("я", domain.MaxCompletionNoteLength+1)
	published := fakeActivities{a: domain.Activity{IsPublished: true}}

	cases := map[string]struct {
		children   fakeChildren
		activities fakeActivities
		note       *string
		want       error
	}{
		"not my child":  {fakeChildren{err: errs.ErrChildNotFound}, published, nil, errs.ErrChildNotFound},
		"no activity":   {fakeChildren{}, fakeActivities{err: errs.ErrActivityNotFound}, nil, errs.ErrActivityNotFound},
		"unpublished":   {fakeChildren{}, fakeActivities{}, nil, errs.ErrActivityNotFound},
		"note too long": {fakeChildren{}, published, &long, errs.ErrValidation},
	}

	for name, tc := range cases {
		comp := &fakeCompletions{}

		_, err := newUC(tc.children, tc.activities, comp).
			Execute(context.Background(), Request{ChildID: "c", ActivityID: "a", Note: tc.note})
		if !errors.Is(err, tc.want) || comp.called {
			t.Errorf("%s: err=%v called=%v", name, err, comp.called)
		}
	}
}

func TestExecute_PlanDay(t *testing.T) {
	plans := &fakePlans{day: 3}

	c, err := newUCWithPlans(fakeChildren{}, fakeActivities{a: domain.Activity{IsPublished: true}}, &fakeCompletions{}, plans).
		Execute(context.Background(), Request{UserID: "u", ChildID: "c", ActivityID: "a"})
	if err != nil || c.PlanDay == nil || *c.PlanDay != 3 || plans.marked != "a@2026-10-04" {
		t.Fatalf("got %+v %v, marked %q", c, err, plans.marked)
	}

	c, err = newUC(fakeChildren{}, fakeActivities{a: domain.Activity{IsPublished: true}}, &fakeCompletions{}).
		Execute(context.Background(), Request{UserID: "u", ChildID: "c", ActivityID: "a", Pro: true})
	if err != nil || c.PlanDay != nil {
		t.Fatalf("not a plan day: %+v %v", c, err)
	}

	if _, err := newUC(fakeChildren{}, fakeActivities{a: domain.Activity{IsPublished: true}}, &fakeCompletions{}).
		Execute(context.Background(), Request{UserID: "u", ChildID: "c", ActivityID: "a"}); !errors.Is(err, errs.ErrNeedsPro) {
		t.Fatalf("free plan, not a plan day: %v", err)
	}
}
