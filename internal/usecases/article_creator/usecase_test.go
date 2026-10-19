package articlecreator

import (
	"context"
	"errors"
	"testing"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

const goalID = "2b6f0cc9-0f3e-4b1a-9a7e-5d8c3e2f1a00"

type mockRepo struct{ got domain.Article }

func (m *mockRepo) Create(_ context.Context, a domain.Article) (domain.Article, error) {
	m.got = a

	return a, nil
}

type fakeGoals struct{ active int }

func (f fakeGoals) CountActive(context.Context, []string) (int, error) { return f.active, nil }

func valid() domain.Article {
	return domain.Article{
		TitleUz: " Sarlavha ", TitleRu: "Заголовок", TitleEn: "Title", BodyUz: "b", BodyRu: "б", BodyEn: "b",
		ReadMinutes: 3, MinAge: 2, MaxAge: 5, GoalID: goalID,
	}
}

func TestExecute_DefaultsAndTrims(t *testing.T) {
	repo := &mockRepo{}

	if _, err := New(nil, repo, fakeGoals{1}).Execute(context.Background(), valid()); err != nil {
		t.Fatal(err)
	}

	if repo.got.Status != domain.ArticleStatusInReview || repo.got.TitleUz != "Sarlavha" {
		t.Fatalf("got %+v", repo.got)
	}
}

func TestExecute_Invalid(t *testing.T) {
	cases := map[string]func(a *domain.Article){
		"no english title": func(a *domain.Article) { a.TitleEn = " " },
		"no russian body":  func(a *domain.Article) { a.BodyRu = "" },
		"bad goal":         func(a *domain.Article) { a.GoalID = "sleep" },
		"bad photo":        func(a *domain.Article) { p := "x"; a.PhotoID = &p },
		"age range":        func(a *domain.Article) { a.MinAge = 5; a.MaxAge = 3 },
		"too old":          func(a *domain.Article) { a.MaxAge = 7 },
		"read time 0":      func(a *domain.Article) { a.ReadMinutes = 0 },
		"read time 61":     func(a *domain.Article) { a.ReadMinutes = 61 },
		"status":           func(a *domain.Article) { a.Status = "draft" },
	}

	for name, mutate := range cases {
		a := valid()
		mutate(&a)

		if _, err := New(nil, &mockRepo{}, fakeGoals{1}).Execute(context.Background(), a); !errors.Is(err, errs.ErrValidation) {
			t.Errorf("%s: want validation error, got %v", name, err)
		}
	}

	if _, err := New(nil, &mockRepo{}, fakeGoals{0}).Execute(context.Background(), valid()); !errors.Is(err, errs.ErrUnknownGoals) {
		t.Errorf("inactive goal: %v", err)
	}
}
