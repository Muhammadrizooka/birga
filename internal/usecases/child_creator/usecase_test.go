package childcreator

import (
	"context"
	"errors"
	"testing"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

type fakeTx struct{}

func (fakeTx) InTx(ctx context.Context, h func(context.Context) error) error { return h(ctx) }

type fakeRepo struct {
	got       domain.Child
	linked    [2]string
	createErr error
	linkErr   error
	existing  int
}

func (f *fakeRepo) List(context.Context, domain.ChildFilter) ([]domain.Child, int, error) {
	return nil, f.existing, nil
}

func (f *fakeRepo) Create(_ context.Context, c domain.Child) (domain.Child, error) {
	f.got = c
	c.ID = "c1"

	return c, f.createErr
}

func (f *fakeRepo) AddParent(_ context.Context, childID, userID string) error {
	f.linked = [2]string{childID, userID}

	return f.linkErr
}

type fakeGoals struct{ active int }

func (f fakeGoals) CountActive(_ context.Context, ids []string) (int, error) {
	if f.active < 0 {
		return len(ids), nil
	}

	return f.active, nil
}

type fakePlans struct {
	started string
	err     error
}

func (f *fakePlans) Start(_ context.Context, c domain.Child) error {
	f.started = c.ID

	return f.err
}

const goalID = "2b6f0cc9-0f3e-4b1a-9a7e-5d8c3e2f1a00"

var goals = []string{goalID}

func newUseCase(repo *fakeRepo, plans *fakePlans, active int) *UseCase {
	return New(nil, fakeTx{}, repo, fakeGoals{active: active}, plans)
}

func TestExecute(t *testing.T) {
	repo := &fakeRepo{}
	blank := "  "

	plans := &fakePlans{}

	got, err := newUseCase(repo, plans, -1).Execute(context.Background(), "u1", false,
		domain.Child{Name: " Amir ", AgeMonths: 36, Gender: " Male", PhotoID: &blank, GoalIDs: []string{goalID, goalID}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if got.ID != "c1" || repo.got.Name != "Amir" || repo.got.Gender != domain.GenderMale || repo.got.PhotoID != nil {
		t.Fatalf("stored %+v, returned %+v", repo.got, got)
	}

	if repo.linked != [2]string{"c1", "u1"} || plans.started != "c1" || len(repo.got.GoalIDs) != 1 {
		t.Fatalf("linked %v, plan for %q, goals %v", repo.linked, plans.started, repo.got.GoalIDs)
	}
}

func TestExecute_Invalid(t *testing.T) {
	bad := "x"

	for name, c := range map[string]domain.Child{
		"no name":  {AgeMonths: 36, Gender: domain.GenderMale, GoalIDs: goals},
		"old":      {Name: "A", AgeMonths: 156, Gender: domain.GenderMale, GoalIDs: goals},
		"negative": {Name: "A", AgeMonths: -1, Gender: domain.GenderMale, GoalIDs: goals},
		"gender":   {Name: "A", AgeMonths: 36, Gender: "other", GoalIDs: goals},
		"photo id": {Name: "A", AgeMonths: 36, Gender: domain.GenderFemale, PhotoID: &bad, GoalIDs: goals},
		"no goals": {Name: "A", AgeMonths: 36, Gender: domain.GenderFemale},
		"bad goal": {Name: "A", AgeMonths: 36, Gender: domain.GenderFemale, GoalIDs: []string{"speech"}},
	} {
		repo := &fakeRepo{}
		if _, err := newUseCase(repo, &fakePlans{}, -1).Execute(context.Background(), "u1", false, c); !errors.Is(err, errs.ErrValidation) {
			t.Errorf("%s: expected validation error, got %v", name, err)
		}

		if repo.got.Name != "" {
			t.Errorf("%s: repo was called", name)
		}
	}
}

func TestExecute_LinkFails(t *testing.T) {
	repo := &fakeRepo{linkErr: errs.ErrUserNotFound}

	if _, err := newUseCase(repo, &fakePlans{}, -1).Execute(context.Background(), "u1", false,
		domain.Child{Name: "A", AgeMonths: 36, Gender: domain.GenderMale, GoalIDs: goals}); !errors.Is(err, errs.ErrUserNotFound) {
		t.Fatalf("expected user not found, got %v", err)
	}
}

func TestExecute_UnknownGoal(t *testing.T) {
	repo := &fakeRepo{}

	if _, err := newUseCase(repo, &fakePlans{}, 0).Execute(context.Background(), "u1", false,
		domain.Child{Name: "A", AgeMonths: 36, Gender: domain.GenderMale, GoalIDs: goals}); !errors.Is(err, errs.ErrUnknownGoals) || repo.got.Name != "" {
		t.Fatalf("got %v", err)
	}
}

func TestExecute_FreePlanOneChild(t *testing.T) {
	child := domain.Child{Name: "Aziz", AgeMonths: 24, Gender: "male", GoalIDs: goals}

	if _, err := newUseCase(&fakeRepo{existing: 1}, &fakePlans{}, -1).Execute(context.Background(), "u1", false, child); !errors.Is(err, errs.ErrNeedsPro) {
		t.Errorf("free plan, second child: %v", err)
	}

	if _, err := newUseCase(&fakeRepo{existing: 1}, &fakePlans{}, -1).Execute(context.Background(), "u1", true, child); err != nil {
		t.Errorf("pro, second child: %v", err)
	}
}
