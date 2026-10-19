package postcreator

import (
	"context"
	"errors"
	"testing"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

const testID = "2b6f0cc9-0f3e-4b1a-9a7e-5d8c3e2f1a00"

type mockRepo struct{ got domain.Post }

func (m *mockRepo) Create(_ context.Context, p domain.Post) (domain.Post, error) {
	m.got = p

	return p, nil
}

type fakeGoals struct{ active int }

func (f fakeGoals) CountActive(context.Context, []string) (int, error) { return f.active, nil }

type fakeMedia struct {
	contentType string
	err         error
}

func (f fakeMedia) Get(context.Context, string) (domain.Media, error) {
	return domain.Media{ContentType: f.contentType}, f.err
}

func valid() domain.Post {
	return domain.Post{TitleUz: "a", TitleRu: "b", TitleEn: "c", VideoID: testID, GoalID: testID, MinAge: 3, MaxAge: 5, Duration: 4.2}
}

func TestExecute(t *testing.T) {
	repo := &mockRepo{}
	if _, err := New(nil, repo, fakeGoals{1}, fakeMedia{contentType: "video/mp4"}).Execute(context.Background(), valid()); err != nil ||
		repo.got.Status != domain.ArticleStatusInReview {
		t.Fatalf("valid: %v %+v", err, repo.got)
	}

	invalid := map[string]func(p *domain.Post){
		"no title":     func(p *domain.Post) { p.TitleRu = "" },
		"bad video id": func(p *domain.Post) { p.VideoID = "x" },
		"bad age":      func(p *domain.Post) { p.MinAge = 1 },
		"zero length":  func(p *domain.Post) { p.Duration = 0 },
		"too long":     func(p *domain.Post) { p.Duration = 601 },
		"bad status":   func(p *domain.Post) { p.Status = "draft" },
	}

	for name, mutate := range invalid {
		p := valid()
		mutate(&p)

		if _, err := New(nil, &mockRepo{}, fakeGoals{1}, fakeMedia{contentType: "video/mp4"}).Execute(context.Background(), p); !errors.Is(err, errs.ErrValidation) {
			t.Errorf("%s: %v", name, err)
		}
	}

	for name, m := range map[string]fakeMedia{
		"an image":      {contentType: "image/png"},
		"missing media": {err: errs.ErrMediaNotFound},
	} {
		if _, err := New(nil, &mockRepo{}, fakeGoals{1}, m).Execute(context.Background(), valid()); !errors.Is(err, errs.ErrVideoNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}

	if _, err := New(nil, &mockRepo{}, fakeGoals{0}, fakeMedia{contentType: "video/mp4"}).Execute(context.Background(), valid()); !errors.Is(err, errs.ErrUnknownGoals) {
		t.Errorf("inactive goal: %v", err)
	}
}
