package plangetter

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
	"gitlab.com/loyihalar/birga/backend/pkg/logger"
)

type childRepo interface {
	GetForParent(ctx context.Context, id, userID string) (domain.Child, error)
}

type userRepo interface {
	Get(ctx context.Context, id string) (domain.User, error)
}

type planRepo interface {
	Themes(ctx context.Context) ([]domain.PlanTheme, error)
	LockChild(ctx context.Context, childID string) error
	Items(ctx context.Context, childID string) ([]domain.PlanItem, []bool, time.Time, error)
	Pick(ctx context.Context, p domain.PlanPick) (domain.Activity, error)
	Replace(ctx context.Context, childID string, startedOn time.Time, items []domain.PlanItem) error
	SetActivity(ctx context.Context, childID string, day int, activityID string) error
	CompletionDays(ctx context.Context, childID string, since time.Time) (map[string][]time.Time, error)
}

type txRunner interface {
	InTx(ctx context.Context, h func(context.Context) error) error
}

// UseCase returns a child's 7-day plan, making one the first time it is asked for.
type UseCase struct {
	l        logger.Logger
	tx       txRunner
	children childRepo
	users    userRepo
	plans    planRepo
	now      func() time.Time
}

// New creates a new plan getter use case.
func New(l logger.Logger, tx txRunner, children childRepo, users userRepo, plans planRepo) *UseCase {
	return &UseCase{l: l, tx: tx, children: children, users: users, plans: plans, now: time.Now}
}

// Execute returns the child's plan with each day's status. With restart, or when the child has no
// plan yet, a new plan starting today replaces the old one. A day not yet done whose activity was
// unpublished or deleted since gets a new activity. Only the child's parents may ask
// (errs.ErrChildNotFound otherwise); no activity for the child's age is errs.ErrNoRecommendation.
func (uc *UseCase) Execute(ctx context.Context, userID, childID string, restart bool) (domain.Plan, error) {
	child, err := uc.children.GetForParent(ctx, childID, userID)
	if err != nil {
		return domain.Plan{}, err
	}

	user, err := uc.users.Get(ctx, userID)
	if err != nil {
		return domain.Plan{}, err
	}

	var (
		today = domain.LocalDay(uc.now())
		pick  = domain.PlanPick{
			ChildID:       child.ID,
			Age:           min(max(child.Age, domain.MinChildAge), domain.MaxChildAge),
			PreferGoalIDs: user.GoalIDs,
		}
		plan = domain.Plan{ChildID: child.ID}
	)

	err = uc.tx.InTx(ctx, func(ctx context.Context) error {
		if err := uc.plans.LockChild(ctx, child.ID); err != nil {
			return err
		}

		items, usable, startedOn, err := uc.plans.Items(ctx, child.ID)
		if err != nil {
			return err
		}

		if restart || len(items) == 0 {
			if items, err = uc.build(ctx, pick); err != nil {
				return err
			}

			startedOn, usable = today, nil

			if err := uc.plans.Replace(ctx, child.ID, startedOn, items); err != nil {
				return err
			}

			logger.WithContext(uc.l, ctx).Info("plan started", zap.String("child_id", child.ID), zap.Bool("restart", restart))
		}

		done, err := uc.plans.CompletionDays(ctx, child.ID, startedOn)
		if err != nil {
			return err
		}

		days, _ := domain.BuildPlanDays(items, startedOn, today, done)

		if items, err = uc.repair(ctx, pick, items, usable, days); err != nil {
			return err
		}

		plan.StartedOn = startedOn
		plan.Days, plan.Completed = domain.BuildPlanDays(items, startedOn, today, done)

		return nil
	})
	if err != nil {
		return domain.Plan{}, err
	}

	return plan, nil
}

// build picks an activity for every theme, different ones while the catalogue allows.
func (uc *UseCase) build(ctx context.Context, pick domain.PlanPick) ([]domain.PlanItem, error) {
	themes, err := uc.plans.Themes(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]domain.PlanItem, 0, len(themes))
	pick.ExcludeIDs = make([]string, 0, len(themes))

	for _, th := range themes {
		a, err := uc.pick(ctx, pick, th)
		if err != nil {
			return nil, err
		}

		items = append(items, domain.PlanItem{Theme: th, Activity: a})
		pick.ExcludeIDs = append(pick.ExcludeIDs, a.ID)
	}

	return items, nil
}

// repair gives a day that is not done yet a new activity when its own was unpublished or deleted.
func (uc *UseCase) repair(ctx context.Context, pick domain.PlanPick, items []domain.PlanItem, usable []bool, days []domain.PlanDay) ([]domain.PlanItem, error) {
	pick.ExcludeIDs = make([]string, 0, len(items))
	for _, it := range items {
		pick.ExcludeIDs = append(pick.ExcludeIDs, it.Activity.ID)
	}

	for i := range usable {
		if usable[i] || days[i].Status == domain.PlanDayDone {
			continue
		}

		a, err := uc.pick(ctx, pick, items[i].Theme)
		if err != nil {
			return nil, err
		}

		if err := uc.plans.SetActivity(ctx, pick.ChildID, items[i].Theme.Day, a.ID); err != nil {
			return nil, err
		}

		items[i].Activity = a
		pick.ExcludeIDs = append(pick.ExcludeIDs, a.ID)
	}

	return items, nil
}

// pick chooses the activity for one theme, repeating an activity already in the plan only when
// the catalogue for the child's age has nothing else.
func (uc *UseCase) pick(ctx context.Context, pick domain.PlanPick, th domain.PlanTheme) (domain.Activity, error) {
	pick.Day, pick.GoalID = th.Day, th.GoalID

	a, err := uc.plans.Pick(ctx, pick)
	if errors.Is(err, errs.ErrNoRecommendation) && len(pick.ExcludeIDs) > 0 {
		pick.ExcludeIDs = nil
		a, err = uc.plans.Pick(ctx, pick)
	}

	return a, err
}
