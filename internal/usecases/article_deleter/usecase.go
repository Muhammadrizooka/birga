package articledeleter

import (
	"context"

	"go.uber.org/zap"

	"gitlab.com/loyihalar/birga/backend/pkg/logger"
)

type articleRepo interface {
	Delete(ctx context.Context, id string) error
}

// UseCase soft-deletes an article.
type UseCase struct {
	l    logger.Logger
	repo articleRepo
}

// New creates a new article deleter use case.
func New(l logger.Logger, repo articleRepo) *UseCase {
	return &UseCase{
		l:    l,
		repo: repo,
	}
}

// Execute marks the article deleted. It disappears from every read, from the daily article and
// from the users' article history.
func (uc *UseCase) Execute(ctx context.Context, id string) error {
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}

	logger.WithContext(uc.l, ctx).Info("article deleted", zap.String("id", id))

	return nil
}
