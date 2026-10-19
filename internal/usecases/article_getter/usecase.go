package articlegetter

import (
	"context"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
	"gitlab.com/loyihalar/birga/backend/pkg/logger"
)

type articleRepo interface {
	Get(ctx context.Context, id, readerID string) (domain.Article, error)
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
