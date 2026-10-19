package dailyarticle

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

// fakeRepo stores daily picks in memory; published lists the published article ids in pick order.
type fakeRepo struct {
	published []string
	days      map[time.Time]string
}

func (f *fakeRepo) LockDaily(context.Context, string) error { return nil }

func (f *fakeRepo) DailyArticleID(_ context.Context, _ string, day time.Time) (string, bool, error) {
	id, ok := f.days[day]
	if !ok || !slices.Contains(f.published, id) {
		return "", false, nil
	}

	return id, true, nil
}

func (f *fakeRepo) PickDaily(_ context.Context, _ string, exclude []string) (string, bool, error) {
	for _, id := range f.published {
		if !slices.Contains(exclude, id) && !slices.Contains(mapValues(f.days), id) {
			return id, true, nil
		}
	}

	for _, id := range f.published {
		if !slices.Contains(exclude, id) {
			return id, true, nil
		}
	}

	return "", false, nil
}

func (f *fakeRepo) SetDailyArticle(_ context.Context, _ string, day time.Time, id string) error {
	f.days[day] = id

	return nil
}

func (f *fakeRepo) Get(_ context.Context, id, _ string) (domain.Article, error) {
	return domain.Article{ID: id}, nil
}

func (f *fakeRepo) DailyHistory(context.Context, string, time.Time, int, int) ([]domain.DailyArticle, int, error) {
	return nil, 0, nil
}

func mapValues(m map[time.Time]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}

	return out
}

func newUC(repo *fakeRepo, now time.Time) *UseCase {
	uc := New(nil, fakeTx{}, repo)
	uc.now = func() time.Time { return now }

	return uc
}

func TestExecute_StableAndTomorrowBecomesToday(t *testing.T) {
	repo := &fakeRepo{published: []string{"a", "b", "c"}, days: map[time.Time]string{}}
	monday := time.Date(2026, 10, 19, 10, 0, 0, 0, domain.Location)

	d, err := newUC(repo, monday).Execute(context.Background(), "u")
	if err != nil || d.Today.Article.ID != "a" || d.Tomorrow == nil || d.Tomorrow.ID != "b" {
		t.Fatalf("first call: %+v %v", d, err)
	}

	again, _ := newUC(repo, monday.Add(5*time.Hour)).Execute(context.Background(), "u")
	if again.Today.Article.ID != "a" || again.Tomorrow.ID != "b" {
		t.Fatalf("same day changed: %+v", again)
	}

	tuesday, _ := newUC(repo, monday.Add(24*time.Hour)).Execute(context.Background(), "u")
	if tuesday.Today.Article.ID != "b" || tuesday.Tomorrow == nil || tuesday.Tomorrow.ID != "c" {
		t.Fatalf("tomorrow's pick must show the next day: %+v", tuesday)
	}
}

func TestExecute_ReplacesUnpublishedAndSingleArticle(t *testing.T) {
	repo := &fakeRepo{published: []string{"a", "b"}, days: map[time.Time]string{}}
	now := time.Date(2026, 10, 19, 10, 0, 0, 0, domain.Location)

	if _, err := newUC(repo, now).Execute(context.Background(), "u"); err != nil {
		t.Fatal(err)
	}

	// Today's "a" is unpublished; only tomorrow's "b" is left, so today takes it and there is no tomorrow.
	repo.published = []string{"b"}

	d, err := newUC(repo, now).Execute(context.Background(), "u")
	if err != nil || d.Today.Article.ID != "b" || d.Tomorrow != nil {
		t.Fatalf("got %+v %v", d, err)
	}

	repo.published = nil
	if _, err := newUC(repo, now).Execute(context.Background(), "u"); !errors.Is(err, errs.ErrNoArticle) {
		t.Fatalf("no article: %v", err)
	}
}
