package proplan

import (
	"context"
	"errors"
	"testing"
	"time"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

type fakeUsers struct{ until *time.Time }

func (f *fakeUsers) Get(_ context.Context, id string) (domain.User, error) {
	return domain.User{ID: id, ProUntil: f.until}, nil
}

func (f *fakeUsers) SetProUntil(_ context.Context, id string, until *time.Time) (domain.User, error) {
	f.until = until

	return domain.User{ID: id, ProUntil: until}, nil
}

func TestPurchase(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	users := &fakeUsers{}
	uc := New(nil, users, 16000)
	uc.now = func() time.Time { return now }
	p := domain.Principal{UserID: "u1"}

	got, err := uc.Purchase(context.Background(), p, " Payme ")
	if err != nil || got.PaymentMethod != "payme" || got.AmountUZS != 16000 || got.Paid || !users.until.Equal(now.AddDate(0, 1, 0)) {
		t.Fatalf("first month: %+v %v", got, err)
	}

	if _, err := uc.Purchase(context.Background(), p, "click"); err != nil || !users.until.Equal(now.AddDate(0, 2, 0)) {
		t.Fatalf("second month adds to the end: %v %v", users.until, err)
	}

	past := now.AddDate(0, -3, 0)
	users.until = &past

	if _, err := uc.Purchase(context.Background(), p, "click"); err != nil || !users.until.Equal(now.AddDate(0, 1, 0)) {
		t.Fatalf("expired Pro counts from now: %v %v", users.until, err)
	}

	if _, err := uc.Purchase(context.Background(), p, "uzcard"); !errors.Is(err, errs.ErrValidation) {
		t.Fatalf("unknown method: %v", err)
	}
}
