package dbstore

import (
	"context"
	"errors"
	"testing"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

func TestMediaRepo_CreateAndUserPhoto(t *testing.T) {
	s := newTestStore(t)

	inRollbackTx(t, s, func(ctx context.Context) {
		id := "5b3c8a0e-1f2d-4c6b-9e7a-2d4f6a8c0b1e"

		m, err := s.Media().Create(ctx, domain.Media{ID: id, Key: "media/" + id + ".png", ContentType: "image/png", Size: 42})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		if m.ID != id || m.Key != "media/"+id+".png" || m.Size != 42 || m.CreatedAt.IsZero() {
			t.Fatalf("unexpected media: %+v", m)
		}

		if got, err := s.Media().Get(ctx, id); err != nil || got.Key != m.Key {
			t.Fatalf("Get: %+v, %v", got, err)
		}

		if _, err := s.Media().Get(ctx, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, errs.ErrMediaNotFound) {
			t.Fatalf("Get unknown: %v", err)
		}

		u, err := s.User().Create(ctx, domain.User{PhoneNumber: strPtr("+998900000009"), PhotoID: &id})
		if err != nil || *u.PhotoID != id {
			t.Fatalf("user with photo: %+v, %v", u, err)
		}

		if _, err := s.sqlClientByCtx(ctx).Exec(ctx, "SAVEPOINT sp"); err != nil {
			t.Fatalf("savepoint: %v", err)
		}

		unknown := "00000000-0000-0000-0000-000000000000"
		if _, err := s.User().Update(ctx, u.ID, domain.UserUpdate{PhotoID: &unknown}); !errors.Is(err, errs.ErrPhotoNotFound) {
			t.Fatalf("expected ErrPhotoNotFound, got %v", err)
		}

		if _, err := s.sqlClientByCtx(ctx).Exec(ctx, "ROLLBACK TO SAVEPOINT sp"); err != nil {
			t.Fatalf("rollback to savepoint: %v", err)
		}
	})
}
