package dailytip

import (
	"context"
	"time"

	"go.uber.org/zap"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
	"gitlab.com/loyihalar/birga/backend/pkg/logger"
)

type tipRepo interface {
	LockDaily(ctx context.Context, userID string) error
	DailyTipID(ctx context.Context, userID string, day time.Time) (string, bool, error)
	PickDaily(ctx context.Context, userID string) (string, bool, error)
	SetDailyTip(ctx context.Context, userID string, day time.Time, tipID string) error
	Get(ctx context.Context, id string) (domain.Tip, error)
}

type txRunner interface {
	InTx(ctx context.Context, h func(context.Context) error) error
}

// UseCase gives a user one tip per day ("Kun maslahati").
type UseCase struct {
	l    logger.Logger
	tx   txRunner
	tips tipRepo
	now  func() time.Time
}

// New creates a new daily tip use case.
func New(l logger.Logger, tx txRunner, tips tipRepo) *UseCase {
	return &UseCase{l: l, tx: tx, tips: tips, now: time.Now}
}

// Execute returns today's tip, picking and storing it the first time the day is asked for. A
// stored pick that was unpublished or deleted since is replaced. No published tip is
// errs.ErrNoTip.
func (uc *UseCase) Execute(ctx context.Context, userID string) (domain.DailyTip, error) {
	var (
		today = domain.LocalDay(uc.now())
		id    string
	)

	err := uc.tx.InTx(ctx, func(ctx context.Context) error {
		if err := uc.tips.LockDaily(ctx, userID); err != nil {
			return err
		}

		var (
			ok  bool
			err error
		)

		if id, ok, err = uc.tips.DailyTipID(ctx, userID, today); err != nil || ok {
			return err
		}

		if id, ok, err = uc.tips.PickDaily(ctx, userID); err != nil {
			return err
		}

		if !ok {
			return errs.ErrNoTip
		}

		logger.WithContext(uc.l, ctx).Info("daily tip picked", zap.String("tip_id", id), zap.Time("day", today))

		return uc.tips.SetDailyTip(ctx, userID, today, id)
	})
	if err != nil {
		return domain.DailyTip{}, err
	}

	t, err := uc.tips.Get(ctx, id)
	if err != nil {
		return domain.DailyTip{}, err
	}

	return domain.DailyTip{Day: today, Tip: t}, nil
}
