package childmilestones

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
	"gitlab.com/loyihalar/birga/backend/pkg/logger"
)

type childRepo interface {
	GetForParent(ctx context.Context, id, userID string) (domain.Child, error)
}

type milestoneRepo interface {
	Stages(ctx context.Context) ([]int, error)
	ForChild(ctx context.Context, childID string, ageMonths int) ([]domain.ChildMilestone, error)
	GetForChild(ctx context.Context, id, childID string) (domain.ChildMilestone, error)
	Mark(ctx context.Context, childID, milestoneID, userID string) error
	Unmark(ctx context.Context, childID, milestoneID string) error
	CountAchieved(ctx context.Context, childID string, from, to time.Time) (int, error)
}

type activityRepo interface {
	ListByIDs(ctx context.Context, ids []string, publishedOnly bool) ([]domain.Activity, error)
}

type completionRepo interface {
	CountBetween(ctx context.Context, childID string, from, to time.Time) (int, error)
}

// noChild stands for "no child given" in GetForChild: no child has this id, so nothing is ticked.
const noChild = "00000000-0000-0000-0000-000000000000"

// UseCase shows a child's development milestones ("Ko‘nikmalar", Figma 27, 44, 45), lets the
// parents tick them and sums up a month.
type UseCase struct {
	l           logger.Logger
	children    childRepo
	milestones  milestoneRepo
	activities  activityRepo
	completions completionRepo
	now         func() time.Time
}

// New creates a new child milestones use case.
func New(l logger.Logger, children childRepo, milestones milestoneRepo, activities activityRepo, completions completionRepo) *UseCase {
	return &UseCase{l: l, children: children, milestones: milestones, activities: activities, completions: completions, now: time.Now}
}

// List returns the published milestones of one stage for the child, grouped by domain, with the
// child's ticks. ageMonths 0 means the child's stage: the highest stage at or below the child's
// age (years * 12). Free-plan users (pro false) get them without advice.
func (uc *UseCase) List(ctx context.Context, userID string, pro bool, childID string, ageMonths int) (domain.ChildMilestones, error) {
	if ageMonths != 0 && (ageMonths < domain.MinMilestoneAgeMonths || ageMonths > domain.MaxMilestoneAgeMonths) {
		return domain.ChildMilestones{}, errs.Errf(errs.ErrValidation, "age_months must be between %d and %d",
			domain.MinMilestoneAgeMonths, domain.MaxMilestoneAgeMonths)
	}

	child, err := uc.children.GetForParent(ctx, childID, userID)
	if err != nil {
		return domain.ChildMilestones{}, err
	}

	stages, err := uc.milestones.Stages(ctx)
	if err != nil {
		return domain.ChildMilestones{}, err
	}

	out := domain.ChildMilestones{AgeMonths: ageMonths, Stages: stages, Domains: map[string][]domain.ChildMilestone{}}
	if out.AgeMonths == 0 {
		out.AgeMonths = domain.StageFor(stages, child.Age*12) //nolint:mnd // months in a year
	}

	if out.AgeMonths == 0 {
		return out, nil
	}

	items, err := uc.milestones.ForChild(ctx, child.ID, out.AgeMonths)
	if err != nil {
		return domain.ChildMilestones{}, err
	}

	for _, it := range items {
		if !pro {
			it.Milestone = it.WithoutAdvice()
		}

		out.Domains[it.Domain] = append(out.Domains[it.Domain], it)
		out.Total++

		if it.Achieved {
			out.Achieved++
		}
	}

	return out, nil
}

// Get returns one published milestone with its advice and related published activities. With
// childID (one of the user's children) it carries the child's tick. Free-plan users get it
// without advice and activities.
func (uc *UseCase) Get(ctx context.Context, userID string, pro bool, id, childID string) (domain.MilestoneDetail, error) {
	if uuid.Validate(id) != nil {
		return domain.MilestoneDetail{}, errs.ErrMilestoneNotFound
	}

	forChild := noChild

	if childID != "" {
		if uuid.Validate(childID) != nil {
			return domain.MilestoneDetail{}, errs.Errf(errs.ErrValidation, "child_id must be a child id (UUID), got %q", childID)
		}

		child, err := uc.children.GetForParent(ctx, childID, userID)
		if err != nil {
			return domain.MilestoneDetail{}, err
		}

		forChild = child.ID
	}

	m, err := uc.milestones.GetForChild(ctx, id, forChild)
	if err != nil {
		return domain.MilestoneDetail{}, err
	}

	if m.Status != domain.ArticleStatusPublished {
		return domain.MilestoneDetail{}, errs.ErrMilestoneNotFound
	}

	out := domain.MilestoneDetail{ChildMilestone: m, Activities: []domain.Activity{}}
	if !pro {
		out.Milestone = m.WithoutAdvice()

		return out, nil
	}

	if len(m.ActivityIDs) > 0 {
		if out.Activities, err = uc.activities.ListByIDs(ctx, m.ActivityIDs, true); err != nil {
			return domain.MilestoneDetail{}, err
		}
	}

	return out, nil
}

// Mark ticks a published milestone for the child ("O‘zlashtirdi"); ticking it again keeps the
// first time. Any parent of the child may tick, on any plan.
func (uc *UseCase) Mark(ctx context.Context, userID, childID, milestoneID string) error {
	if err := uc.check(ctx, userID, childID, milestoneID); err != nil {
		return err
	}

	if err := uc.milestones.Mark(ctx, childID, milestoneID, userID); err != nil {
		return err
	}

	logger.WithContext(uc.l, ctx).Info("milestone ticked", zap.String("child_id", childID), zap.String("milestone_id", milestoneID))

	return nil
}

// Unmark removes the child's tick.
func (uc *UseCase) Unmark(ctx context.Context, userID, childID, milestoneID string) error {
	if err := uc.check(ctx, userID, childID, milestoneID); err != nil {
		return err
	}

	return uc.milestones.Unmark(ctx, childID, milestoneID)
}

func (uc *UseCase) check(ctx context.Context, userID, childID, milestoneID string) error {
	if uuid.Validate(milestoneID) != nil {
		return errs.ErrMilestoneNotFound
	}

	if _, err := uc.children.GetForParent(ctx, childID, userID); err != nil {
		return err
	}

	m, err := uc.milestones.GetForChild(ctx, milestoneID, childID)
	if err != nil {
		return err
	}

	if m.Status != domain.ArticleStatusPublished {
		return errs.ErrMilestoneNotFound
	}

	return nil
}

// Report sums up the child's month ("2026-10"; empty means this month, Uzbekistan time): the
// milestones ticked in it and the activities done. It needs Birga Pro.
func (uc *UseCase) Report(ctx context.Context, userID string, pro bool, childID, month string) (domain.MonthlyReport, error) {
	if !pro {
		return domain.MonthlyReport{}, errs.ErrNeedsPro
	}

	today := domain.LocalDay(uc.now())
	start := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)

	if month != "" {
		t, err := time.Parse("2006-01", month)
		if err != nil {
			return domain.MonthlyReport{}, errs.Errf(errs.ErrValidation, "month must look like 2026-10, got %q", month)
		}

		start = t
	}

	if _, err := uc.children.GetForParent(ctx, childID, userID); err != nil {
		return domain.MonthlyReport{}, err
	}

	end := start.AddDate(0, 1, 0)

	// Local midnights of the month, as instants.
	from := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, domain.Location)
	to := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, domain.Location)

	ticked, err := uc.milestones.CountAchieved(ctx, childID, from, to)
	if err != nil {
		return domain.MonthlyReport{}, err
	}

	done, err := uc.completions.CountBetween(ctx, childID, start, end.AddDate(0, 0, -1))
	if err != nil {
		return domain.MonthlyReport{}, err
	}

	return domain.MonthlyReport{Month: start, NewMilestones: ticked, Completions: done}, nil
}
