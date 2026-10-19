package postgetter

import (
	"context"
	"time"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
	"gitlab.com/loyihalar/birga/backend/pkg/logger"
)

type postRepo interface {
	Get(ctx context.Context, id string) (domain.Post, error)
	WasDaily(ctx context.Context, userID, postID string, day time.Time) (bool, error)
}

// UseCase returns one post by id.
type UseCase struct {
	l    logger.Logger
	repo postRepo
	now  func() time.Time
}

// New creates a new post getter use case.
func New(l logger.Logger, repo postRepo) *UseCase {
	return &UseCase{l: l, repo: repo, now: time.Now}
}

// Execute returns the post in any status (admin callers).
func (uc *UseCase) Execute(ctx context.Context, id string) (domain.Post, error) {
	return uc.repo.Get(ctx, id)
}

// ForUser returns a published post to a signed-in app user. Roles that may open any post (paid
// users, admins) get every published post; the others only their own daily videos of today or
// earlier days (errs.ErrPostNotOpened otherwise).
func (uc *UseCase) ForUser(ctx context.Context, userID string, role domain.UserRole, id string) (domain.Post, error) {
	p, err := uc.repo.Get(ctx, id)
	if err != nil {
		return domain.Post{}, err
	}

	if p.Status != domain.ArticleStatusPublished {
		return domain.Post{}, errs.ErrPostNotFound
	}

	if domain.CanOpenAnyPost(role) {
		return p, nil
	}

	ok, err := uc.repo.WasDaily(ctx, userID, id, domain.LocalDay(uc.now()))
	if err != nil {
		return domain.Post{}, err
	}

	if !ok {
		return domain.Post{}, errs.ErrPostNotOpened
	}

	return p, nil
}
