package articlegetter

import (
	"context"
	"time"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
	"gitlab.com/loyihalar/birga/backend/pkg/logger"
)

type articleRepo interface {
	Get(ctx context.Context, id, readerID string) (domain.Article, error)
	WasDaily(ctx context.Context, userID, articleID string, day time.Time) (bool, error)
}

// UseCase returns one article by id.
type UseCase struct {
	l    logger.Logger
	repo articleRepo
}

// New creates a new article getter use case.
func New(l logger.Logger, repo articleRepo) *UseCase {
	return &UseCase{
		l:    l,
		repo: repo,
	}
}

// Execute returns the article. With a readerID (a signed-in app user) only a published article is
// found and IsRead is filled; without one (admin callers) every status is returned.
func (uc *UseCase) Execute(ctx context.Context, id, readerID string) (domain.Article, error) {
	a, err := uc.repo.Get(ctx, id, readerID)
	if err != nil {
		return domain.Article{}, err
	}

	if readerID != "" && a.Status != domain.ArticleStatusPublished {
		return domain.Article{}, errs.ErrArticleNotFound
	}

	return a, nil
}

// ForUser returns a published article to a signed-in app user, with IsRead. Pro users open every
// published article; the others only their own daily articles of today or earlier days
// (errs.ErrArticleNotOpened otherwise; tomorrow's article opens tomorrow).
func (uc *UseCase) ForUser(ctx context.Context, userID string, pro bool, id string) (domain.Article, error) {
	a, err := uc.Execute(ctx, id, userID)
	if err != nil || pro {
		return a, err
	}

	ok, err := uc.repo.WasDaily(ctx, userID, id, domain.LocalDay(time.Now()))
	if err != nil {
		return domain.Article{}, err
	}

	if !ok {
		return domain.Article{}, errs.ErrArticleNotOpened
	}

	return a, nil
}
