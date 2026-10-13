package streakgetter

import (
	"context"
	"errors"
	"testing"
	"time"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

func oct(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC) }

type fakeChildren struct{ err error }

func (f fakeChildren) GetForParent(_ context.Context, id, _ string) (domain.Child, error) {
	return domain.Child{ID: id}, f.err
}

type fakePlans struct{ called bool }

func (f *fakePlans) DoneDays(context.Context, string) ([]time.Time, int, time.Time, error) {
	f.called = true

	return []time.Time{oct(3), oct(2)}, 2, oct(1), nil
}

func newUC(children fakeChildren, plans *fakePlans) *UseCase {
	uc := New(nil, children, plans)
	uc.now = func() time.Time { return time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC) }

	return uc
}

func TestExecute(t *testing.T) {
	s, err := newUC(fakeChildren{}, &fakePlans{}).Execute(context.Background(), "u", "c")
	if err != nil || s.Current != 2 || s.CompletedToday || s.Total != 2 {
		t.Fatalf("streak: %+v %v", s, err)
	}

	plans := &fakePlans{}
	if _, err := newUC(fakeChildren{err: errs.ErrChildNotFound}, plans).Execute(context.Background(), "u", "c"); !errors.Is(err, errs.ErrChildNotFound) || plans.called {
		t.Fatalf("not my child: %v called=%v", err, plans.called)
	}
}

func TestDays(t *testing.T) {
	days, err := newUC(fakeChildren{}, &fakePlans{}).Days(context.Background(), "u", "c", time.Time{}, time.Time{})
	if err != nil || len(days) != 4 {
		t.Fatalf("got %v %v", days, err)
	}

	want := []string{domain.StreakDayMissed, domain.StreakDayDone, domain.StreakDayDone, domain.StreakDayToday}
	for i, d := range days {
		if d.Status != want[i] || !d.Day.Equal(oct(i+1)) {
			t.Fatalf("day %d: %+v", i, d)
		}
	}

	for name, r := range map[string][2]time.Time{
		"reversed": {oct(5), oct(4)},
		"too long": {oct(1).AddDate(-1, 0, -1), oct(1)},
	} {
		if _, err := newUC(fakeChildren{}, &fakePlans{}).Days(context.Background(), "u", "c", r[0], r[1]); !errors.Is(err, errs.ErrValidation) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}
