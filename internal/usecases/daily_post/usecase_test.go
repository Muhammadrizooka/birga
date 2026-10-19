package dailypost

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

type fakeTx struct{}

func (fakeTx) InTx(ctx context.Context, h func(context.Context) error) error { return h(ctx) }

// fakeRepo picks the published posts in order, unshown first.
type fakeRepo struct {
	published []string
	days      map[time.Time]string
}

func (f *fakeRepo) LockDaily(context.Context, string) error { return nil }

func (f *fakeRepo) DailyPostID(_ context.Context, _ string, day time.Time) (string, bool, error) {
	id, ok := f.days[day]

	return id, ok && slices.Contains(f.published, id), nil
}

func (f *fakeRepo) PickDaily(_ context.Context, _ string, exclude []string) (string, bool, error) {
	for _, id := range f.published {
		if !slices.Contains(exclude, id) {
			return id, true, nil
		}
	}

	return "", false, nil
}

func (f *fakeRepo) SetDailyPost(_ context.Context, _ string, day time.Time, id string) error {
	f.days[day] = id

	return nil
}

func (f *fakeRepo) Get(_ context.Context, id string) (domain.Post, error) {
	return domain.Post{ID: id}, nil
}

func (f *fakeRepo) DailyHistory(context.Context, string, time.Time, int, int) ([]domain.DailyPost, int, error) {
	return nil, 0, nil
}

func TestExecute(t *testing.T) {
	repo := &fakeRepo{published: []string{"a", "b", "c", "d"}, days: map[time.Time]string{}}
	uc := New(nil, fakeTx{}, repo)
	uc.now = func() time.Time { return time.Date(2026, 10, 19, 23, 0, 0, 0, domain.Location) }

	d, err := uc.Execute(context.Background(), "u")
	if err != nil || d.Today.Post.ID != "a" || len(d.Next) != domain.MaxNextPosts || d.Next[0].ID != "b" || d.Next[1].ID != "c" {
		t.Fatalf("first: %+v %v", d, err)
	}

	// The stored pick stays for the day even when the pick order changes.
	repo.published = []string{"d", "a"}
	if d, _ := uc.Execute(context.Background(), "u"); d.Today.Post.ID != "a" || len(d.Next) != 1 || d.Next[0].ID != "d" {
		t.Fatalf("same day: %+v", d)
	}

	// An unpublished pick is replaced.
	repo.published = []string{"d"}
	if d, _ := uc.Execute(context.Background(), "u"); d.Today.Post.ID != "d" || len(d.Next) != 0 {
		t.Fatalf("replaced: %+v", d)
	}

	repo.published = nil
	if _, err := uc.Execute(context.Background(), "u"); !errors.Is(err, errs.ErrNoPost) {
		t.Fatalf("no post: %v", err)
	}
}
