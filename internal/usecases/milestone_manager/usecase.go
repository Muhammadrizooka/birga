package milestonemanager

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
	"gitlab.com/loyihalar/birga/backend/pkg/logger"
)

type milestoneRepo interface {
	Create(ctx context.Context, m domain.Milestone) (domain.Milestone, error)
	Get(ctx context.Context, id string) (domain.Milestone, error)
	Update(ctx context.Context, id string, upd domain.MilestoneUpdate) (domain.Milestone, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, f domain.MilestoneFilter) ([]domain.Milestone, int, error)
}

type activityRepo interface {
	ListByIDs(ctx context.Context, ids []string, publishedOnly bool) ([]domain.Activity, error)
}

// UseCase manages the development milestones for the admin.
type UseCase struct {
	l          logger.Logger
	repo       milestoneRepo
	activities activityRepo
}

// New creates a new milestone manager use case.
func New(l logger.Logger, repo milestoneRepo, activities activityRepo) *UseCase {
	return &UseCase{l: l, repo: repo, activities: activities}
}

// Create validates and stores a milestone. An empty status means in_review.
func (uc *UseCase) Create(ctx context.Context, m domain.Milestone) (domain.Milestone, error) {
	m.Domain = strings.ToLower(strings.TrimSpace(m.Domain))
	m.Status = strings.ToLower(strings.TrimSpace(m.Status))

	if m.Status == "" {
		m.Status = domain.ArticleStatusInReview
	}

	upd := domain.MilestoneUpdate{
		Domain: &m.Domain, AgeMonths: &m.AgeMonths, SortOrder: &m.SortOrder, Status: &m.Status,
		TextUz: &m.TextUz, TextRu: &m.TextRu, TextEn: &m.TextEn,
		MeaningUz: &m.MeaningUz, MeaningRu: &m.MeaningRu, MeaningEn: &m.MeaningEn,
		HowToUz: &m.HowToUz, HowToRu: &m.HowToRu, HowToEn: &m.HowToEn,
		SpecialistUz: &m.SpecialistUz, SpecialistRu: &m.SpecialistRu, SpecialistEn: &m.SpecialistEn,
		ActivityIDs: m.ActivityIDs,
	}
	if upd.ActivityIDs == nil {
		upd.ActivityIDs = []string{}
	}

	if err := uc.validate(ctx, &upd); err != nil {
		return domain.Milestone{}, err
	}

	m.ActivityIDs = upd.ActivityIDs
	m.TextUz, m.TextRu, m.TextEn = *upd.TextUz, *upd.TextRu, *upd.TextEn
	m.MeaningUz, m.MeaningRu, m.MeaningEn = *upd.MeaningUz, *upd.MeaningRu, *upd.MeaningEn
	m.HowToUz, m.HowToRu, m.HowToEn = *upd.HowToUz, *upd.HowToRu, *upd.HowToEn
	m.SpecialistUz, m.SpecialistRu, m.SpecialistEn = *upd.SpecialistUz, *upd.SpecialistRu, *upd.SpecialistEn

	created, err := uc.repo.Create(ctx, m)
	if err != nil {
		return domain.Milestone{}, err
	}

	logger.WithContext(uc.l, ctx).Info("milestone created", zap.String("id", created.ID))

	return created, nil
}

// Get returns a milestone in any status.
func (uc *UseCase) Get(ctx context.Context, id string) (domain.Milestone, error) {
	return uc.repo.Get(ctx, id)
}

// Update applies the given fields; each follows the create rules.
func (uc *UseCase) Update(ctx context.Context, id string, upd domain.MilestoneUpdate) (domain.Milestone, error) {
	if upd.IsEmpty() {
		return domain.Milestone{}, errs.Errf(errs.ErrValidation, "nothing to update")
	}

	if upd.Domain != nil {
		v := strings.ToLower(strings.TrimSpace(*upd.Domain))
		upd.Domain = &v
	}

	if upd.Status != nil {
		v := strings.ToLower(strings.TrimSpace(*upd.Status))
		upd.Status = &v
	}

	if err := uc.validate(ctx, &upd); err != nil {
		return domain.Milestone{}, err
	}

	updated, err := uc.repo.Update(ctx, id, upd)
	if err != nil {
		return domain.Milestone{}, err
	}

	logger.WithContext(uc.l, ctx).Info("milestone updated", zap.String("id", id))

	return updated, nil
}

// Delete soft-deletes the milestone.
func (uc *UseCase) Delete(ctx context.Context, id string) error {
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}

	logger.WithContext(uc.l, ctx).Info("milestone deleted", zap.String("id", id))

	return nil
}

// List returns a page of milestones and the total number of matches.
func (uc *UseCase) List(ctx context.Context, f domain.MilestoneFilter) ([]domain.Milestone, int, error) {
	f.Search = strings.TrimSpace(f.Search)
	f.Status = strings.ToLower(strings.TrimSpace(f.Status))
	f.Domain = strings.ToLower(strings.TrimSpace(f.Domain))

	switch {
	case utf8.RuneCountInString(f.Search) > domain.MaxArticleSearchLength:
		return nil, 0, errs.Errf(errs.ErrValidation, "q must be at most %d characters", domain.MaxArticleSearchLength)
	case f.Status != "" && !domain.IsKnownArticleStatus(f.Status):
		return nil, 0, statusError()
	case f.Domain != "" && !domain.IsMilestoneDomain(f.Domain):
		return nil, 0, domainError()
	case f.AgeMonths != 0 && !ageOK(f.AgeMonths):
		return nil, 0, ageError()
	}

	return uc.repo.List(ctx, f)
}

// validate trims the given texts and checks every given field.
func (uc *UseCase) validate(ctx context.Context, upd *domain.MilestoneUpdate) error {
	required, advice := upd.Texts()
	for _, s := range append(required, advice...) {
		if *s != nil {
			v := strings.TrimSpace(**s)
			*s = &v
		}
	}

	for _, s := range required {
		switch {
		case *s != nil && **s == "":
			return errs.Errf(errs.ErrValidation, "text_uz, text_ru and text_en are required")
		case *s != nil && utf8.RuneCountInString(**s) > domain.MaxMilestoneTextLength:
			return errs.Errf(errs.ErrValidation, "each text is at most %d characters", domain.MaxMilestoneTextLength)
		}
	}

	for _, s := range advice {
		if *s != nil && utf8.RuneCountInString(**s) > domain.MaxMilestoneAdviceLength {
			return errs.Errf(errs.ErrValidation, "each advice text is at most %d characters", domain.MaxMilestoneAdviceLength)
		}
	}

	switch {
	case upd.Domain != nil && !domain.IsMilestoneDomain(*upd.Domain):
		return domainError()
	case upd.AgeMonths != nil && !ageOK(*upd.AgeMonths):
		return ageError()
	case upd.Status != nil && !domain.IsKnownArticleStatus(*upd.Status):
		return statusError()
	}

	if upd.ActivityIDs == nil {
		return nil
	}

	upd.ActivityIDs = domain.NormalizeIDs(upd.ActivityIDs)
	if len(upd.ActivityIDs) > domain.MaxMilestoneActivities {
		return errs.Errf(errs.ErrValidation, "activity_ids holds at most %d ids", domain.MaxMilestoneActivities)
	}

	for _, id := range upd.ActivityIDs {
		if uuid.Validate(id) != nil {
			return errs.Errf(errs.ErrValidation, "activity_ids must be activity ids (UUID), got %q", id)
		}
	}

	found, err := uc.activities.ListByIDs(ctx, upd.ActivityIDs, false)
	if err != nil {
		return err
	}

	if len(found) != len(upd.ActivityIDs) {
		return errs.ErrUnknownActivities
	}

	return nil
}

func ageOK(m int) bool {
	return m >= domain.MinMilestoneAgeMonths && m <= domain.MaxMilestoneAgeMonths
}

func ageError() error {
	return errs.Errf(errs.ErrValidation, "age_months must be between %d and %d",
		domain.MinMilestoneAgeMonths, domain.MaxMilestoneAgeMonths)
}

func domainError() error {
	return errs.Errf(errs.ErrValidation, "domain must be one of %s", strings.Join(domain.MilestoneDomains, ", "))
}

func statusError() error {
	return errs.Errf(errs.ErrValidation, "status must be %s, %s or %s",
		domain.ArticleStatusPublished, domain.ArticleStatusInReview, domain.ArticleStatusUnpublished)
}
