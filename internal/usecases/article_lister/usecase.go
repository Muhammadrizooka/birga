package articlelister

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
	"gitlab.com/loyihalar/birga/backend/pkg/logger"
)

type articleRepo interface {
	List(ctx context.Context, f domain.ArticleFilter) ([]domain.Article, int, error)
}

// UseCase lists articles with filtering and pagination.
type UseCase struct {
	l    logger.Logger
	repo articleRepo
}

// New creates a new article lister use case.
func New(l logger.Logger, repo articleRepo) *UseCase {
	return &UseCase{
		l:    l,
		repo: repo,
	}
}

// Execute returns a page of articles and the total number of matches.
func (uc *UseCase) Execute(ctx context.Context, f domain.ArticleFilter) ([]domain.Article, int, error) {
	f.Search = strings.TrimSpace(f.Search)
	f.Status = strings.ToLower(strings.TrimSpace(f.Status))

	switch {
	case utf8.RuneCountInString(f.Search) > domain.MaxArticleSearchLength:
		return nil, 0, errs.Errf(errs.ErrValidation, "q must be at most %d characters", domain.MaxArticleSearchLength)
	case f.Status != "" && !domain.IsKnownArticleStatus(f.Status):
		return nil, 0, errs.Errf(errs.ErrValidation, "status must be %s, %s or %s",
			domain.ArticleStatusPublished, domain.ArticleStatusInReview, domain.ArticleStatusUnpublished)
	case f.GoalID != "" && uuid.Validate(f.GoalID) != nil:
		return nil, 0, errs.Errf(errs.ErrValidation, "goal_id must be a goal id (UUID), got %q", f.GoalID)
	case f.Age != 0 && (f.Age < domain.MinChildAge || f.Age > domain.MaxChildAge):
		return nil, 0, errs.Errf(errs.ErrValidation, "age must be between %d and %d", domain.MinChildAge, domain.MaxChildAge)
	}

	return uc.repo.List(ctx, f)
}
