package tipmanager

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
	"gitlab.com/loyihalar/birga/backend/pkg/logger"
)

type tipRepo interface {
	Create(ctx context.Context, t domain.Tip) (domain.Tip, error)
	Get(ctx context.Context, id string) (domain.Tip, error)
	Update(ctx context.Context, id string, upd domain.TipUpdate) (domain.Tip, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, f domain.TipFilter) ([]domain.Tip, int, error)
}

type topicRepo interface {
	Get(ctx context.Context, id string) (domain.Topic, error)
}

// UseCase manages tips ("Kun maslahati") for the admin and lists them for the app.
type UseCase struct {
	l      logger.Logger
	repo   tipRepo
	topics topicRepo
}

// New creates a new tip manager use case.
func New(l logger.Logger, repo tipRepo, topics topicRepo) *UseCase {
	return &UseCase{l: l, repo: repo, topics: topics}
}

// Create validates and stores a tip. An empty status means in_review; topic_id must be an
// active topic.
func (uc *UseCase) Create(ctx context.Context, t domain.Tip) (domain.Tip, error) {
	for _, s := range []*string{&t.TextUz, &t.TextRu, &t.TextEn, &t.TopicID} {
		*s = strings.TrimSpace(*s)
	}

	t.Status = strings.ToLower(strings.TrimSpace(t.Status))
	if t.Status == "" {
		t.Status = domain.ArticleStatusInReview
	}

	switch {
	case t.TextUz == "" || t.TextRu == "" || t.TextEn == "":
		return domain.Tip{}, errs.Errf(errs.ErrValidation, "text_uz, text_ru and text_en are required")
	case !validText(&t.TextUz) || !validText(&t.TextRu) || !validText(&t.TextEn):
		return domain.Tip{}, errs.Errf(errs.ErrValidation, "each text is at most %d characters", domain.MaxTipTextLength)
	case t.MinAge < domain.MinChildAge || t.MaxAge > domain.MaxChildAge || t.MinAge > t.MaxAge:
		return domain.Tip{}, errs.Errf(errs.ErrValidation, "age range must be within %d..%d and min_age <= max_age",
			domain.MinChildAge, domain.MaxChildAge)
	case !domain.IsKnownArticleStatus(t.Status):
		return domain.Tip{}, statusError()
	}

	if err := uc.checkTopic(ctx, t.TopicID); err != nil {
		return domain.Tip{}, err
	}

	created, err := uc.repo.Create(ctx, t)
	if err != nil {
		return domain.Tip{}, err
	}

	logger.WithContext(uc.l, ctx).Info("tip created", zap.String("id", created.ID))

	return created, nil
}

// Get returns a tip. With appOnly (app callers) only a published tip is found.
func (uc *UseCase) Get(ctx context.Context, id string, appOnly bool) (domain.Tip, error) {
	t, err := uc.repo.Get(ctx, id)
	if err != nil {
		return domain.Tip{}, err
	}

	if appOnly && t.Status != domain.ArticleStatusPublished {
		return domain.Tip{}, errs.ErrTipNotFound
	}

	return t, nil
}

// Update applies the given fields; each follows the create rules.
func (uc *UseCase) Update(ctx context.Context, id string, upd domain.TipUpdate) (domain.Tip, error) {
	if upd.IsEmpty() {
		return domain.Tip{}, errs.Errf(errs.ErrValidation, "nothing to update")
	}

	for _, s := range []**string{&upd.TextUz, &upd.TextRu, &upd.TextEn, &upd.TopicID} {
		if *s != nil {
			v := strings.TrimSpace(**s)
			*s = &v
		}
	}

	if upd.Status != nil {
		v := strings.ToLower(strings.TrimSpace(*upd.Status))
		upd.Status = &v
	}

	inRange := func(a *int) bool { return a == nil || (*a >= domain.MinChildAge && *a <= domain.MaxChildAge) }

	switch {
	case blank(upd.TextUz) || blank(upd.TextRu) || blank(upd.TextEn):
		return domain.Tip{}, errs.Errf(errs.ErrValidation, "text_uz, text_ru and text_en cannot be empty")
	case !validText(upd.TextUz) || !validText(upd.TextRu) || !validText(upd.TextEn):
		return domain.Tip{}, errs.Errf(errs.ErrValidation, "each text is at most %d characters", domain.MaxTipTextLength)
	case !inRange(upd.MinAge) || !inRange(upd.MaxAge) || (upd.MinAge != nil && upd.MaxAge != nil && *upd.MinAge > *upd.MaxAge):
		return domain.Tip{}, errs.Errf(errs.ErrValidation, "age range must be within %d..%d and min_age <= max_age",
			domain.MinChildAge, domain.MaxChildAge)
	case upd.Status != nil && !domain.IsKnownArticleStatus(*upd.Status):
		return domain.Tip{}, statusError()
	}

	if upd.TopicID != nil {
		if err := uc.checkTopic(ctx, *upd.TopicID); err != nil {
			return domain.Tip{}, err
		}
	}

	updated, err := uc.repo.Update(ctx, id, upd)
	if err != nil {
		return domain.Tip{}, err
	}

	logger.WithContext(uc.l, ctx).Info("tip updated", zap.String("id", id))

	return updated, nil
}

// Delete soft-deletes the tip.
func (uc *UseCase) Delete(ctx context.Context, id string) error {
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}

	logger.WithContext(uc.l, ctx).Info("tip deleted", zap.String("id", id))

	return nil
}

// List returns a page of tips and the total number of matches.
func (uc *UseCase) List(ctx context.Context, f domain.TipFilter) ([]domain.Tip, int, error) {
	f.Search = strings.TrimSpace(f.Search)
	f.Status = strings.ToLower(strings.TrimSpace(f.Status))

	switch {
	case utf8.RuneCountInString(f.Search) > domain.MaxArticleSearchLength:
		return nil, 0, errs.Errf(errs.ErrValidation, "q must be at most %d characters", domain.MaxArticleSearchLength)
	case f.Status != "" && !domain.IsKnownArticleStatus(f.Status):
		return nil, 0, statusError()
	case f.TopicID != "" && uuid.Validate(f.TopicID) != nil:
		return nil, 0, errs.Errf(errs.ErrValidation, "topic_id must be a topic id (UUID), got %q", f.TopicID)
	case f.Age != 0 && (f.Age < domain.MinChildAge || f.Age > domain.MaxChildAge):
		return nil, 0, errs.Errf(errs.ErrValidation, "age must be between %d and %d", domain.MinChildAge, domain.MaxChildAge)
	}

	return uc.repo.List(ctx, f)
}

func (uc *UseCase) checkTopic(ctx context.Context, id string) error {
	if uuid.Validate(id) != nil {
		return errs.Errf(errs.ErrValidation, "topic_id must be a topic id (UUID) from GET /v1/topics")
	}

	_, err := uc.topics.Get(ctx, id)
	if errors.Is(err, errs.ErrTopicNotFound) {
		return errs.ErrUnknownTopic
	}

	return err
}

func statusError() error {
	return errs.Errf(errs.ErrValidation, "status must be %s, %s or %s",
		domain.ArticleStatusPublished, domain.ArticleStatusInReview, domain.ArticleStatusUnpublished)
}

func blank(s *string) bool { return s != nil && *s == "" }

func validText(s *string) bool {
	return s == nil || utf8.RuneCountInString(*s) <= domain.MaxTipTextLength
}
