package proplan

import (
	"context"
	"time"

	"go.uber.org/zap"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/pkg/logger"
)

type userRepo interface {
	Get(ctx context.Context, id string) (domain.User, error)
	SetProUntil(ctx context.Context, id string, until *time.Time) (domain.User, error)
}

// UseCase tells a user about Birga Pro and lets an admin give or end it by date.
type UseCase struct {
	l        logger.Logger
	users    userRepo
	priceUZS int
	now      func() time.Time
}

// New creates a new Pro plan use case. priceUZS is the monthly price shown on the offer.
func New(l logger.Logger, users userRepo, priceUZS int) *UseCase {
	return &UseCase{l: l, users: users, priceUZS: priceUZS, now: time.Now}
}

// Offer returns the price and the caller's plan; p is the signed-in user.
func (uc *UseCase) Offer(ctx context.Context, p domain.Principal) (domain.ProOffer, error) {
	u, err := uc.users.Get(ctx, p.UserID)
	if err != nil {
		return domain.ProOffer{}, err
	}

	plan := domain.PlanFree
	if domain.IsPro(p.Role, u.ProUntil, uc.now()) {
		plan = domain.PlanPro
	}

	return domain.ProOffer{PriceUZS: uc.priceUZS, Plan: plan, ProUntil: u.ProUntil}, nil
}

// Grant sets the date the user's Pro ends; nil ends it now. A date in the past also ends it.
func (uc *UseCase) Grant(ctx context.Context, userID string, until *time.Time) (domain.User, error) {
	u, err := uc.users.SetProUntil(ctx, userID, until)
	if err != nil {
		return domain.User{}, err
	}

	logger.WithContext(uc.l, ctx).Info("pro until set", zap.String("user_id", userID), zap.Timep("pro_until", until))

	return u, nil
}
