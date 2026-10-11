package childupdater

import (
	"context"
	"errors"
	"testing"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

type fakeChildren struct {
	getErr  error
	got     domain.ChildUpdate
	updated bool
}

func (f *fakeChildren) GetForParent(_ context.Context, id, _ string) (domain.Child, error) {
	return domain.Child{ID: id}, f.getErr
}

func (f *fakeChildren) Update(_ context.Context, id string, upd domain.ChildUpdate) (domain.Child, error) {
	f.got, f.updated = upd, true

	return domain.Child{ID: id}, nil
}

func ptr[T any](v T) *T { return &v }

func TestExecute_Normalizes(t *testing.T) {
	repo := &fakeChildren{}

	_, err := New(nil, repo).Execute(context.Background(), "u1", "c1", domain.ChildUpdate{
		Name: ptr(" Amir "), Age: ptr(5), Gender: ptr(" Male "), PhotoID: ptr(" "),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if *repo.got.Name != "Amir" || *repo.got.Age != 5 || *repo.got.Gender != "male" || *repo.got.PhotoID != "" {
		t.Fatalf("unexpected update: %+v", repo.got)
	}
}

func TestExecute_Validation(t *testing.T) {
	for name, upd := range map[string]domain.ChildUpdate{
		"empty":      {},
		"blank name": {Name: ptr("  ")},
		"old":        {Age: ptr(19)},
		"negative":   {Age: ptr(-1)},
		"gender":     {Gender: ptr("other")},
		"photo":      {PhotoID: ptr("x")},
	} {
		repo := &fakeChildren{}
		if _, err := New(nil, repo).Execute(context.Background(), "u1", "c1", upd); !errors.Is(err, errs.ErrValidation) || repo.updated {
			t.Fatalf("%s: expected validation error before any write, got %v", name, err)
		}
	}
}

func TestExecute_NotParent(t *testing.T) {
	repo := &fakeChildren{getErr: errs.ErrChildNotFound}

	if _, err := New(nil, repo).Execute(context.Background(), "u1", "c1", domain.ChildUpdate{Age: ptr(4)}); !errors.Is(err, errs.ErrNotFound) || repo.updated {
		t.Fatalf("expected not found without update, got %v", err)
	}
}
