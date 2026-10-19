package postdeleter

import (
	"context"

	"go.uber.org/zap"

	"gitlab.com/loyihalar/birga/backend/pkg/logger"
)

type postRepo interface {
	Delete(ctx context.Context, id string) error
}

// UseCase soft-deletes a post.
type UseCase struct {
	l    logger.Logger
	repo postRepo
}

// New creates a new post deleter use case.
func New(l logger.Logger, repo postRepo) *UseCase {
	return &UseCase{l: l, repo: repo}
}

// Execute marks the post deleted. It disappears from every read, from the daily video and from
// the users' video history. The uploaded video file is kept.
func (uc *UseCase) Execute(ctx context.Context, id string) error {
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}

	logger.WithContext(uc.l, ctx).Info("post deleted", zap.String("id", id))

	return nil
}
