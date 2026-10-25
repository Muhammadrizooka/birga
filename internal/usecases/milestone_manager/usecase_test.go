package milestonemanager

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

const known = "55555555-5555-5555-5555-555555555555"

type fakeRepo struct {
	created domain.Milestone
	updated domain.MilestoneUpdate
}

func (f *fakeRepo) Create(_ context.Context, m domain.Milestone) (domain.Milestone, error) {
	f.created = m

	return m, nil
}

func (f *fakeRepo) Get(context.Context, string) (domain.Milestone, error) {
	return domain.Milestone{}, nil
}

func (f *fakeRepo) Update(_ context.Context, _ string, upd domain.MilestoneUpdate) (domain.Milestone, error) {
	f.updated = upd

	return domain.Milestone{}, nil
}

func (f *fakeRepo) Delete(context.Context, string) error { return nil }

func (f *fakeRepo) List(context.Context, domain.MilestoneFilter) ([]domain.Milestone, int, error) {
	return nil, 0, nil
}

type fakeActivities struct{}

func (fakeActivities) ListByIDs(_ context.Context, ids []string, _ bool) ([]domain.Activity, error) {
	var out []domain.Activity

	for _, id := range ids {
		if id == known {
			out = append(out, domain.Activity{ID: id})
		}
	}

	return out, nil
}

func valid() domain.Milestone {
	return domain.Milestone{Domain: " Social ", AgeMonths: 36, TextUz: " a ", TextRu: "b", TextEn: "c"}
}

func TestCreate(t *testing.T) {
	repo := &fakeRepo{}
	uc := New(nil, repo, fakeActivities{})
	ctx := context.Background()

	m := valid()
	m.ActivityIDs = []string{known, strings.ToUpper(known)}
	m.HowToUz = "  qadam  "

	if _, err := uc.Create(ctx, m); err != nil {
		t.Fatal(err)
	}

	if c := repo.created; c.Domain != "social" || c.Status != domain.ArticleStatusInReview || c.TextUz != "a" ||
		c.HowToUz != "qadam" || len(c.ActivityIDs) != 1 {
		t.Errorf("created %+v", c)
	}

	if _, err := uc.Create(ctx, valid()); err != nil || repo.created.ActivityIDs == nil {
		t.Errorf("no activities: %v %v", repo.created.ActivityIDs, err)
	}

	for name, change := range map[string]func(*domain.Milestone){
		"domain":  func(m *domain.Milestone) { m.Domain = "music" },
		"age":     func(m *domain.Milestone) { m.AgeMonths = 1 },
		"text":    func(m *domain.Milestone) { m.TextRu = " " },
		"long":    func(m *domain.Milestone) { m.TextEn = strings.Repeat("x", domain.MaxMilestoneTextLength+1) },
		"advice":  func(m *domain.Milestone) { m.SpecialistEn = strings.Repeat("x", domain.MaxMilestoneAdviceLength+1) },
		"status":  func(m *domain.Milestone) { m.Status = "draft" },
		"bad id":  func(m *domain.Milestone) { m.ActivityIDs = []string{"nope"} },
		"unknown": func(m *domain.Milestone) { m.ActivityIDs = []string{"66666666-6666-6666-6666-666666666666"} },
	} {
		m := valid()
		change(&m)

		if _, err := uc.Create(ctx, m); !errors.Is(err, errs.ErrValidation) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestUpdate(t *testing.T) {
	repo := &fakeRepo{}
	uc := New(nil, repo, fakeActivities{})
	ctx := context.Background()

	if _, err := uc.Update(ctx, known, domain.MilestoneUpdate{}); !errors.Is(err, errs.ErrValidation) {
		t.Errorf("empty: %v", err)
	}

	empty, status := "", " PUBLISHED "
	if _, err := uc.Update(ctx, known, domain.MilestoneUpdate{MeaningUz: &empty, Status: &status}); err != nil ||
		*repo.updated.Status != "published" || *repo.updated.MeaningUz != "" {
		t.Errorf("clear advice: %+v %v", repo.updated, err)
	}

	if _, err := uc.Update(ctx, known, domain.MilestoneUpdate{TextUz: &empty}); !errors.Is(err, errs.ErrValidation) {
		t.Errorf("empty text: %v", err)
	}
}
