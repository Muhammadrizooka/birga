package tipmanager

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

const topicID = "9a1c7e52-3b4d-4f6a-8e2b-7c5d1f0a9b3e"

type fakeRepo struct {
	created domain.Tip
	got     domain.Tip
}

func (f *fakeRepo) Create(_ context.Context, t domain.Tip) (domain.Tip, error) {
	f.created = t

	return t, nil
}

func (f *fakeRepo) Get(context.Context, string) (domain.Tip, error) { return f.got, nil }

func (f *fakeRepo) Update(_ context.Context, id string, _ domain.TipUpdate) (domain.Tip, error) {
	return domain.Tip{ID: id}, nil
}

func (f *fakeRepo) Delete(context.Context, string) error { return nil }

func (f *fakeRepo) List(context.Context, domain.TipFilter) ([]domain.Tip, int, error) {
	return nil, 0, nil
}

type fakeTopics struct{ missing bool }

func (f fakeTopics) Get(_ context.Context, id string) (domain.Topic, error) {
	if f.missing {
		return domain.Topic{}, errs.ErrTopicNotFound
	}

	return domain.Topic{ID: id}, nil
}

func tip() domain.Tip {
	return domain.Tip{TextUz: " a ", TextRu: "b", TextEn: "c", TopicID: topicID, MinAge: 2, MaxAge: 6}
}

func TestCreate(t *testing.T) {
	repo := &fakeRepo{}

	if _, err := New(nil, repo, fakeTopics{}).Create(context.Background(), tip()); err != nil {
		t.Fatalf("create: %v", err)
	}

	if repo.created.TextUz != "a" || repo.created.Status != domain.ArticleStatusInReview {
		t.Errorf("not normalized: %+v", repo.created)
	}

	cases := map[string]struct {
		edit   func(*domain.Tip)
		topics fakeTopics
		want   error
	}{
		"empty text":    {func(t *domain.Tip) { t.TextEn = " " }, fakeTopics{}, errs.ErrValidation},
		"long text":     {func(t *domain.Tip) { t.TextRu = strings.Repeat("я", domain.MaxTipTextLength+1) }, fakeTopics{}, errs.ErrValidation},
		"bad ages":      {func(t *domain.Tip) { t.MinAge = 5; t.MaxAge = 3 }, fakeTopics{}, errs.ErrValidation},
		"bad status":    {func(t *domain.Tip) { t.Status = "draft" }, fakeTopics{}, errs.ErrValidation},
		"topic not id":  {func(t *domain.Tip) { t.TopicID = "sleep" }, fakeTopics{}, errs.ErrValidation},
		"unknown topic": {func(*domain.Tip) {}, fakeTopics{missing: true}, errs.ErrUnknownTopic},
	}

	for name, tc := range cases {
		tp := tip()
		tc.edit(&tp)

		if _, err := New(nil, &fakeRepo{}, tc.topics).Create(context.Background(), tp); !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", name, err, tc.want)
		}
	}
}

func TestGet_AppOnlyHidesUnpublished(t *testing.T) {
	uc := New(nil, &fakeRepo{got: domain.Tip{Status: domain.ArticleStatusInReview}}, fakeTopics{})

	if _, err := uc.Get(context.Background(), "x", true); !errors.Is(err, errs.ErrTipNotFound) {
		t.Errorf("app get of draft: %v", err)
	}

	if _, err := uc.Get(context.Background(), "x", false); err != nil {
		t.Errorf("admin get of draft: %v", err)
	}
}
