package topicmanager

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
	"gitlab.com/loyihalar/birga/backend/pkg/logger"
)

type topicRepo interface {
	Create(ctx context.Context, t domain.Topic) (domain.Topic, error)
	Get(ctx context.Context, id string) (domain.Topic, error)
	List(ctx context.Context) ([]domain.Topic, error)
	Update(ctx context.Context, id string, upd domain.TopicUpdate) (domain.Topic, error)
	Delete(ctx context.Context, id string) error
}

// UseCase manages the topics list (tags of tips, articles and videos). Names follow the goal
// rules: all three languages, at most domain.MaxGoalNameLength characters, unique per language.
type UseCase struct {
	l    logger.Logger
	repo topicRepo
}

// New creates a new topic manager use case.
func New(l logger.Logger, repo topicRepo) *UseCase {
	return &UseCase{l: l, repo: repo}
}

// Create trims the names, requires all three and stores the topic.
func (uc *UseCase) Create(ctx context.Context, t domain.Topic) (domain.Topic, error) {
	t.NameUz, t.NameRu, t.NameEn = strings.TrimSpace(t.NameUz), strings.TrimSpace(t.NameRu), strings.TrimSpace(t.NameEn)

	if !domain.IsValidGoalName(t.NameUz) || !domain.IsValidGoalName(t.NameRu) || !domain.IsValidGoalName(t.NameEn) {
		return domain.Topic{}, errs.Errf(errs.ErrValidation,
			"name_uz, name_ru and name_en are required, at most %d characters each", domain.MaxGoalNameLength)
	}

	created, err := uc.repo.Create(ctx, t)
	if err != nil {
		return domain.Topic{}, err
	}

	logger.WithContext(uc.l, ctx).Info("topic created", zap.String("id", created.ID))

	return created, nil
}

// Get returns an active topic.
func (uc *UseCase) Get(ctx context.Context, id string) (domain.Topic, error) {
	return uc.repo.Get(ctx, id)
}

// List returns every active topic, oldest first.
func (uc *UseCase) List(ctx context.Context) ([]domain.Topic, error) {
	return uc.repo.List(ctx)
}

// Update changes the given names; a given name cannot be empty.
func (uc *UseCase) Update(ctx context.Context, id string, upd domain.TopicUpdate) (domain.Topic, error) {
	if upd.IsEmpty() {
		return domain.Topic{}, errs.Errf(errs.ErrValidation, "nothing to update")
	}

	for _, n := range []**string{&upd.NameUz, &upd.NameRu, &upd.NameEn} {
		if *n == nil {
			continue
		}

		v := strings.TrimSpace(**n)
		if !domain.IsValidGoalName(v) {
			return domain.Topic{}, errs.Errf(errs.ErrValidation,
				"names cannot be empty and are at most %d characters", domain.MaxGoalNameLength)
		}

		*n = &v
	}

	updated, err := uc.repo.Update(ctx, id, upd)
	if err != nil {
		return domain.Topic{}, err
	}

	logger.WithContext(uc.l, ctx).Info("topic updated", zap.String("id", id))

	return updated, nil
}

// Delete soft-deletes the topic. Content tagged with it keeps the tag.
func (uc *UseCase) Delete(ctx context.Context, id string) error {
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}

	logger.WithContext(uc.l, ctx).Info("topic deleted", zap.String("id", id))

	return nil
}
