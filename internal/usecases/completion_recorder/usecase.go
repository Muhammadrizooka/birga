package completionrecorder

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
	"gitlab.com/loyihalar/birga/backend/pkg/logger"
)

type childRepo interface {
	GetForParent(ctx context.Context, id, userID string) (domain.Child, error)
}

type activityRepo interface {
	Get(ctx context.Context, id string) (domain.Activity, error)
}

type completionRepo interface {
	Create(ctx context.Context, c domain.Completion) (domain.Completion, bool, error)
}

type planRepo interface {
	LockChild(ctx context.Context, childID string) error
	MarkDone(ctx context.Context, childID, activityID string, on time.Time) (int, error)
}

type txRunner interface {
	InTx(ctx context.Context, h func(context.Context) error) error
}

// Request is one "we did it" tap from a caregiver.
type Request struct {
	UserID     string
	ChildID    string
	ActivityID string
	Note       *string
}

// UseCase records that a child completed an activity today.
type UseCase struct {
	l           logger.Logger
	tx          txRunner
	children    childRepo
	activities  activityRepo
	completions completionRepo
	plans       planRepo
	now         func() time.Time
}

// New creates a new completion recorder use case.
func New(l logger.Logger, tx txRunner, children childRepo, activities activityRepo, completions completionRepo, plans planRepo) *UseCase {
	return &UseCase{l: l, tx: tx, children: children, activities: activities, completions: completions, plans: plans, now: time.Now}
}

// Execute checks that the child is the caller's and the activity is published, then records the
// completion for today (Uzbekistan time). Repeating it the same day returns the same completion and
// replaces the note when a new one is given. A blank note is stored as no note. When the activity
// is the open day of the child's plan and no plan day was done today yet, the day is marked done
// and counts toward the child's streak; Completion.PlanDay then names it.
func (uc *UseCase) Execute(ctx context.Context, req Request) (domain.Completion, error) {
	if req.Note != nil {
		v := strings.TrimSpace(*req.Note)
		req.Note = &v

		if v == "" {
			req.Note = nil
		} else if utf8.RuneCountInString(v) > domain.MaxCompletionNoteLength {
			return domain.Completion{}, errs.Errf(errs.ErrValidation, "note must be at most %d characters",
				domain.MaxCompletionNoteLength)
		}
	}

	if _, err := uc.children.GetForParent(ctx, req.ChildID, req.UserID); err != nil {
		return domain.Completion{}, err
	}

	a, err := uc.activities.Get(ctx, req.ActivityID)
	if err != nil {
		return domain.Completion{}, err
	}

	if !a.IsPublished {
		return domain.Completion{}, errs.ErrActivityNotFound
	}

	var (
		c       domain.Completion
		created bool
		today   = domain.LocalDay(uc.now())
	)

	err = uc.tx.InTx(ctx, func(ctx context.Context) error {
		if err := uc.plans.LockChild(ctx, req.ChildID); err != nil {
			return err
		}

		c, created, err = uc.completions.Create(ctx, domain.Completion{
			ChildID:     req.ChildID,
			ActivityID:  req.ActivityID,
			UserID:      &req.UserID,
			CompletedOn: today,
			Note:        req.Note,
		})
		if err != nil {
			return err
		}

		day, err := uc.plans.MarkDone(ctx, req.ChildID, req.ActivityID, today)
		if err != nil {
			return err
		}

		if day > 0 {
			c.PlanDay = &day
		}

		return nil
	})
	if err != nil {
		return domain.Completion{}, err
	}

	if created {
		logger.WithContext(uc.l, ctx).Info("activity completed",
			zap.String("child_id", c.ChildID), zap.String("activity_id", c.ActivityID))
	}

	return c, nil
}
