package dailyarticle

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
	"gitlab.com/loyihalar/birga/backend/pkg/logger"
)

type articleRepo interface {
	LockDaily(ctx context.Context, userID string) error
	DailyArticleID(ctx context.Context, userID string, day time.Time) (string, bool, error)
	PickDaily(ctx context.Context, userID string, exclude []string) (string, bool, error)
	SetDailyArticle(ctx context.Context, userID string, day time.Time, articleID string) error
	Get(ctx context.Context, id, readerID string) (domain.Article, error)
	DailyHistory(ctx context.Context, userID string, day time.Time, limit, offset int) ([]domain.DailyArticle, int, error)
}

type txRunner interface {
	InTx(ctx context.Context, h func(context.Context) error) error
}

// UseCase gives a user one article per day, picked from the goals and ages of the user's
// children, and lists the articles of earlier days.
type UseCase struct {
	l        logger.Logger
	tx       txRunner
	articles articleRepo
	now      func() time.Time
}

// New creates a new daily article use case.
func New(l logger.Logger, tx txRunner, articles articleRepo) *UseCase {
	return &UseCase{l: l, tx: tx, articles: articles, now: time.Now}
}

// Execute returns today's article and tomorrow's, picking and storing each the first time it is
// asked for. A stored pick that was unpublished or deleted since is replaced. No published
// article at all is errs.ErrNoArticle; Tomorrow is nil when today's is the only one.
func (uc *UseCase) Execute(ctx context.Context, userID string) (domain.DailyArticles, error) {
	var (
		today       = domain.LocalDay(uc.now())
		tomorrow    = today.AddDate(0, 0, 1)
		todayID     string
		tomorrowID  string
		hasTomorrow bool
	)

	err := uc.tx.InTx(ctx, func(ctx context.Context) error {
		if err := uc.articles.LockDaily(ctx, userID); err != nil {
			return err
		}

		var err error

		if tomorrowID, hasTomorrow, err = uc.articles.DailyArticleID(ctx, userID, tomorrow); err != nil {
			return err
		}

		if todayID, err = uc.ensure(ctx, userID, today, tomorrowID, true); err != nil {
			return err
		}

		if hasTomorrow && tomorrowID != todayID {
			return nil
		}

		tomorrowID, err = uc.ensure(ctx, userID, tomorrow, todayID, false)
		hasTomorrow = err == nil

		if errors.Is(err, errs.ErrNoArticle) {
			return nil
		}

		return err
	})
	if err != nil {
		return domain.DailyArticles{}, err
	}

	out := domain.DailyArticles{Today: domain.DailyArticle{Day: today}}

	if out.Today.Article, err = uc.articles.Get(ctx, todayID, userID); err != nil {
		return domain.DailyArticles{}, err
	}

	if hasTomorrow {
		a, err := uc.articles.Get(ctx, tomorrowID, userID)
		if err != nil {
			return domain.DailyArticles{}, err
		}

		out.Tomorrow = &a
	}

	return out, nil
}

// ensure returns the article stored for day, picking and storing one (other than avoid) when
// there is none or it is no longer published. With orAvoided, avoid is still taken when no other
// article is left: today's card is never empty while an article is published.
func (uc *UseCase) ensure(ctx context.Context, userID string, day time.Time, avoid string, orAvoided bool) (string, error) {
	id, ok, err := uc.articles.DailyArticleID(ctx, userID, day)
	if err != nil || (ok && id != avoid) {
		return id, err
	}

	var exclude []string
	if avoid != "" {
		exclude = []string{avoid}
	}

	id, ok, err = uc.articles.PickDaily(ctx, userID, exclude)
	if err == nil && !ok && exclude != nil && orAvoided {
		id, ok, err = uc.articles.PickDaily(ctx, userID, nil)
	}

	switch {
	case err != nil:
		return "", err
	case !ok:
		return "", errs.ErrNoArticle
	}

	if err := uc.articles.SetDailyArticle(ctx, userID, day, id); err != nil {
		return "", err
	}

	logger.WithContext(uc.l, ctx).Info("daily article picked",
		zap.String("article_id", id), zap.Time("day", day))

	return id, nil
}

// History returns one page of the user's daily articles up to today, newest first, and their total.
func (uc *UseCase) History(ctx context.Context, userID string, limit, offset int) ([]domain.DailyArticle, int, error) {
	return uc.articles.DailyHistory(ctx, userID, domain.LocalDay(uc.now()), limit, offset)
}
