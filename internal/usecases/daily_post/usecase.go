package dailypost

import (
	"context"
	"time"

	"go.uber.org/zap"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
	"gitlab.com/loyihalar/birga/backend/pkg/logger"
)

type postRepo interface {
	LockDaily(ctx context.Context, userID string) error
	DailyPostID(ctx context.Context, userID string, day time.Time) (string, bool, error)
	PickDaily(ctx context.Context, userID string, exclude []string) (string, bool, error)
	SetDailyPost(ctx context.Context, userID string, day time.Time, postID string) error
	Get(ctx context.Context, id string) (domain.Post, error)
	DailyHistory(ctx context.Context, userID string, day time.Time, limit, offset int) ([]domain.DailyPost, int, error)
}

type txRunner interface {
	InTx(ctx context.Context, h func(context.Context) error) error
}

// UseCase gives a user one video per day, picked from the goals and ages of the user's
// children, and lists the videos of earlier days.
type UseCase struct {
	l     logger.Logger
	tx    txRunner
	posts postRepo
	now   func() time.Time
}

// New creates a new daily video use case.
func New(l logger.Logger, tx txRunner, posts postRepo) *UseCase {
	return &UseCase{l: l, tx: tx, posts: posts, now: time.Now}
}

// Execute returns today's video, picking and storing it the first time the day is asked for (a
// stored pick that was unpublished or deleted since is replaced), and up to domain.MaxNextPosts
// videos that would come next. No published post is errs.ErrNoPost.
func (uc *UseCase) Execute(ctx context.Context, userID string) (domain.DailyPosts, error) {
	var (
		today = domain.LocalDay(uc.now())
		ids   []string
	)

	err := uc.tx.InTx(ctx, func(ctx context.Context) error {
		if err := uc.posts.LockDaily(ctx, userID); err != nil {
			return err
		}

		id, ok, err := uc.posts.DailyPostID(ctx, userID, today)
		if err != nil {
			return err
		}

		if !ok {
			if id, ok, err = uc.posts.PickDaily(ctx, userID, nil); err != nil {
				return err
			}

			if !ok {
				return errs.ErrNoPost
			}

			if err := uc.posts.SetDailyPost(ctx, userID, today, id); err != nil {
				return err
			}

			logger.WithContext(uc.l, ctx).Info("daily video picked", zap.String("post_id", id))
		}

		ids = append(ids, id)

		// The next videos are a preview, not stored: they are what the pick would give after today's.
		for range domain.MaxNextPosts {
			next, ok, err := uc.posts.PickDaily(ctx, userID, ids)
			if err != nil {
				return err
			}

			if !ok {
				break
			}

			ids = append(ids, next)
		}

		return nil
	})
	if err != nil {
		return domain.DailyPosts{}, err
	}

	out := domain.DailyPosts{Today: domain.DailyPost{Day: today}, Next: make([]domain.Post, 0, len(ids)-1)}

	for i, id := range ids {
		p, err := uc.posts.Get(ctx, id)
		if err != nil {
			return domain.DailyPosts{}, err
		}

		if i == 0 {
			out.Today.Post = p
		} else {
			out.Next = append(out.Next, p)
		}
	}

	return out, nil
}

// History returns one page of the user's daily videos up to today, newest first, and their total.
func (uc *UseCase) History(ctx context.Context, userID string, limit, offset int) ([]domain.DailyPost, int, error) {
	return uc.posts.DailyHistory(ctx, userID, domain.LocalDay(uc.now()), limit, offset)
}
