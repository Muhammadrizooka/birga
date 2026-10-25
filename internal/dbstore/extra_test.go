package dbstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

func TestActivityRepo_Variants(t *testing.T) {
	s := newTestStore(t)

	inRollbackTx(t, s, func(ctx context.Context) {
		in := activity(testGoal(t, s, ctx, "variant test"), 2, 4, true)
		in.EasierUz, in.HarderEn = "oson", "harder"

		a, err := s.Activity().Create(ctx, in)
		if err != nil || a.EasierUz != "oson" || a.HarderEn != "harder" || a.EasierRu != "" {
			t.Fatalf("Create: %+v, %v", a, err)
		}

		upd, err := s.Activity().Update(ctx, a.ID, domain.ActivityUpdate{HarderUz: strPtr("qiyin"), EasierUz: strPtr("")})
		if err != nil || upd.HarderUz != "qiyin" || upd.EasierUz != "" || upd.HarderEn != "harder" {
			t.Fatalf("Update: %+v, %v", upd, err)
		}
	})
}

func TestExtraRepo(t *testing.T) {
	s := newTestStore(t)

	inRollbackTx(t, s, func(ctx context.Context) {
		hideOtherActivities(t, s, ctx)

		var (
			goal     = testGoal(t, s, ctx, "extra goal")
			other    = testGoal(t, s, ctx, "extra other")
			_, child = parentAndChild(t, s, ctx, 3)
			day      = domain.LocalDay(time.Now())
			repo     = s.Extra()
		)

		create := func(goalID string, minAge, maxAge int) domain.Activity {
			a, err := s.Activity().Create(ctx, activity(goalID, minAge, maxAge, true))
			if err != nil {
				t.Fatalf("Create activity: %v", err)
			}

			return a
		}

		done := create(goal, 2, 4)
		fresh := create(goal, 3, 3)
		create(goal, 5, 6) // too old for the child
		create(other, 2, 6)

		if _, _, err := s.Completion().Create(ctx, domain.Completion{ChildID: child.ID, ActivityID: done.ID, CompletedOn: day.AddDate(0, 0, -1)}); err != nil {
			t.Fatalf("Create completion: %v", err)
		}

		pick := domain.ExtraPick{ChildID: child.ID, Age: 3, GoalID: goal}
		if a, err := repo.Pick(ctx, pick, day); err != nil || a.ID != fresh.ID {
			t.Fatalf("never-done first: %s %v", a.ID, err)
		}

		pick.ExcludeIDs = []string{fresh.ID}
		if a, err := repo.Pick(ctx, pick, day); err != nil || a.ID != done.ID {
			t.Fatalf("then done longest ago: %s %v", a.ID, err)
		}

		pick.ExcludeIDs = []string{fresh.ID, done.ID}
		if _, err := repo.Pick(ctx, pick, day); !errors.Is(err, errs.ErrNoExtra) {
			t.Fatalf("none left: %v", err)
		}

		for _, id := range []string{fresh.ID, done.ID, fresh.ID} {
			if err := repo.Add(ctx, child.ID, day, id); err != nil {
				t.Fatalf("Add: %v", err)
			}
		}

		if _, _, err := s.Completion().Create(ctx, domain.Completion{ChildID: child.ID, ActivityID: done.ID, CompletedOn: day}); err != nil {
			t.Fatalf("Create completion: %v", err)
		}

		list, err := repo.ListDay(ctx, child.ID, day)
		if err != nil || len(list) != 2 || list[0].Activity.ID != fresh.ID || list[0].Done || !list[1].Done {
			t.Fatalf("ListDay: %+v %v", list, err)
		}

		if list, err := repo.ListDay(ctx, child.ID, day.AddDate(0, 0, 1)); err != nil || len(list) != 0 {
			t.Fatalf("other day: %+v %v", list, err)
		}
	})
}

func TestActivityRepo_Similar(t *testing.T) {
	s := newTestStore(t)

	inRollbackTx(t, s, func(ctx context.Context) {
		hideOtherActivities(t, s, ctx)

		var (
			goal     = testGoal(t, s, ctx, "similar goal")
			other    = testGoal(t, s, ctx, "similar other")
			_, child = parentAndChild(t, s, ctx, 4)
		)

		create := func(goalID string, minAge, maxAge int) domain.Activity {
			a, err := s.Activity().Create(ctx, activity(goalID, minAge, maxAge, true))
			if err != nil {
				t.Fatalf("Create activity: %v", err)
			}

			return a
		}

		base := create(goal, 2, 6)
		done := create(goal, 3, 5)
		fresh := create(goal, 4, 4)
		young := create(goal, 2, 3)
		create(other, 2, 6)

		if _, _, err := s.Completion().Create(ctx, domain.Completion{ChildID: child.ID, ActivityID: done.ID, CompletedOn: domain.LocalDay(time.Now())}); err != nil {
			t.Fatalf("Create completion: %v", err)
		}

		got, err := s.Activity().Similar(ctx, domain.SimilarFilter{ActivityID: base.ID, GoalIDs: base.GoalIDs, ChildID: child.ID, Age: 4, Limit: 3})
		if err != nil || len(got) != 2 || got[0].ID != fresh.ID || got[1].ID != done.ID {
			t.Fatalf("for child: %+v %v", got, err)
		}

		got, err = s.Activity().Similar(ctx, domain.SimilarFilter{ActivityID: base.ID, GoalIDs: base.GoalIDs, ChildID: child.ID, Age: 4, Limit: 3, ExcludeIDs: []string{fresh.ID}})
		if err != nil || len(got) != 1 || got[0].ID != done.ID {
			t.Fatalf("plan activities left out: %+v %v", got, err)
		}

		got, err = s.Activity().Similar(ctx, domain.SimilarFilter{ActivityID: base.ID, GoalIDs: base.GoalIDs, MinAge: 2, MaxAge: 6, Limit: 10})
		if err != nil || len(got) != 3 {
			t.Fatalf("by age range: %+v %v", got, err)
		}

		got, err = s.Activity().Similar(ctx, domain.SimilarFilter{ActivityID: young.ID, GoalIDs: young.GoalIDs, MinAge: 2, MaxAge: 3, Limit: 10})
		if err != nil || len(got) != 2 {
			t.Fatalf("young range: %+v %v", got, err)
		}
	})
}
