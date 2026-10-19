package articlereader

import (
	"context"

	"gitlab.com/loyihalar/birga/backend/pkg/logger"
)

type articleRepo interface {
	MarkRead(ctx context.Context, userID, articleID string) error
}

// UseCase marks a published article read for a user ("O'qib chiqdim").
type UseCase struct {
	l    logger.Logger
	repo articleRepo
}

// New creates a new article reader use case.
func New(l logger.Logger, repo articleRepo) *UseCase {
	return &UseCase{
		l:    l,
		repo: repo,
	}
}

// Execute marks the article read. Marking it again is not an error; an article that is not
// published is ErrArticleNotFound.
func (uc *UseCase) Execute(ctx context.Context, userID, articleID string) error {
	return uc.repo.MarkRead(ctx, userID, articleID)
}
