package dbstore

import (
	"context"
	"errors"
	"testing"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

func TestGoalRepo_CRUD(t *testing.T) {
	s := newTestStore(t)

	inRollbackTx(t, s, func(ctx context.Context) {
		created, err := s.Goal().Create(ctx, domain.Goal{NameUz: "Nutq test", NameRu: "Речь тест", NameEn: "Speech test"})
		if err != nil || created.ID == "" || created.CreatedAt.IsZero() {
			t.Fatalf("Create: %+v, %v", created, err)
		}

		updated, err := s.Goal().Update(ctx, created.ID, domain.GoalUpdate{NameEn: strPtr("Talking test")})
		if err != nil || updated.NameEn != "Talking test" || updated.NameUz != "Nutq test" {
			t.Fatalf("Update: %+v, %v", updated, err)
		}

		list, err := s.Goal().List(ctx)
		if err != nil || len(list) == 0 || list[len(list)-1].ID != created.ID {
			t.Fatalf("List: %+v, %v", list, err)
		}

		if n, err := s.Goal().CountActive(ctx, []string{created.ID, "00000000-0000-0000-0000-000000000000"}); err != nil || n != 1 {
			t.Fatalf("CountActive: %d, %v", n, err)
		}

		checkGoalDelete(ctx, t, s, created.ID)
	})
}

// checkGoalDelete deletes the goal, checks it is gone, and that its names are free again.
func checkGoalDelete(ctx context.Context, t *testing.T, s *DBStore, id string) {
	t.Helper()

	if err := s.Goal().Delete(ctx, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := s.Goal().Get(ctx, id); !errors.Is(err, errs.ErrGoalNotFound) {
		t.Fatalf("Get after delete: %v", err)
	}

	if err := s.Goal().Delete(ctx, id); !errors.Is(err, errs.ErrGoalNotFound) {
		t.Fatalf("second Delete: %v", err)
	}

	if _, err := s.Goal().Create(ctx, domain.Goal{NameUz: "Nutq test", NameRu: "Речь тест", NameEn: "Talking test"}); err != nil {
		t.Fatalf("Create with freed names: %v", err)
	}
}

func TestGoalRepo_UniqueNames(t *testing.T) {
	s := newTestStore(t)

	inRollbackTx(t, s, func(ctx context.Context) {
		if _, err := s.Goal().Create(ctx, domain.Goal{NameUz: "Harakat test", NameRu: "Моторика тест", NameEn: "Motor test"}); err != nil {
			t.Fatalf("Create: %v", err)
		}

		_, err := s.Goal().Create(ctx, domain.Goal{NameUz: "Boshqa test", NameRu: "Другое тест", NameEn: "MOTOR TEST"})
		if !errors.Is(err, errs.ErrGoalNameTaken) || !errors.Is(err, errs.ErrConflict) {
			t.Fatalf("expected name taken, got %v", err)
		}
	})
}

func TestChildRepo_Goals(t *testing.T) {
	s := newTestStore(t)

	inRollbackTx(t, s, func(ctx context.Context) {
		goal, err := s.Goal().Create(ctx, domain.Goal{NameUz: "Ijtimoiy test", NameRu: "Социальное тест", NameEn: "Social test"})
		if err != nil {
			t.Fatalf("Create goal: %v", err)
		}

		u, err := s.User().Create(ctx, domain.User{
			Username: strPtr("goals_test"), PhoneNumber: strPtr("+998900000099"), Relationship: strPtr(domain.RelationshipNanny),
		})
		if err != nil || *u.Relationship != domain.RelationshipNanny {
			t.Fatalf("Create user: %+v %v", u, err)
		}

		c, err := s.Child().Create(ctx, domain.Child{Name: "Goal", AgeMonths: 36, Gender: domain.GenderMale, GoalIDs: []string{goal.ID}})
		if err != nil || len(c.GoalIDs) != 1 || c.GoalIDs[0] != goal.ID {
			t.Fatalf("Create child: %+v %v", c, err)
		}

		a, err := s.Activity().Create(ctx, activity(goal.ID, 2, 6, true))
		if err != nil {
			t.Fatalf("Create activity: %v", err)
		}

		// Deleting the goal removes it from the child and from the activity.
		if err := s.Goal().Delete(ctx, goal.ID); err != nil {
			t.Fatalf("Delete goal: %v", err)
		}

		got, err := s.Child().Get(ctx, c.ID)
		if err != nil || got.GoalIDs == nil || len(got.GoalIDs) != 0 {
			t.Fatalf("Get child after goal delete: %+v, %v", got, err)
		}

		if got, err := s.Activity().Get(ctx, a.ID); err != nil || len(got.GoalIDs) != 0 {
			t.Fatalf("Get activity after goal delete: %+v, %v", got, err)
		}

		other, _ := s.Goal().Create(ctx, domain.Goal{NameUz: "Til test", NameRu: "Речь тест", NameEn: "Speech test"})

		upd, err := s.Child().Update(ctx, c.ID, domain.ChildUpdate{GoalIDs: &[]string{other.ID}})
		if err != nil || len(upd.GoalIDs) != 1 || upd.GoalIDs[0] != other.ID {
			t.Fatalf("Update child goals: %+v, %v", upd, err)
		}
	})
}
