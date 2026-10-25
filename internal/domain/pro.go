package domain

import "time"

// User plans as the app shows them.
const (
	PlanFree = "free"
	PlanPro  = "pro"
)

// IsPro reports whether a user with this role and pro_until has Birga Pro at now: roles paid_user
// and admin always do, the others while pro_until is after now.
func IsPro(role UserRole, proUntil *time.Time, now time.Time) bool {
	return role == UserRolePaidUser || role == UserRoleAdmin || (proUntil != nil && proUntil.After(now))
}

// ProOffer is what the Pro offer screen needs: the price and the caller's plan.
type ProOffer struct {
	PriceUZS int
	Plan     string
	ProUntil *time.Time
}

// PaymentMethods are the payment types the Pro payment screen offers (Figma 38).
var PaymentMethods = []string{"payme", "click"}

// ProPurchase is the result of the Pro "To‘lash" button. Paid is false while there is no
// payment integration: Pro is turned on without charging.
type ProPurchase struct {
	PaymentMethod string
	AmountUZS     int
	Paid          bool
	User          User
}
