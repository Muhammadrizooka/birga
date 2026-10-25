package rest

import (
	"time"

	"github.com/gin-gonic/gin"

	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

type proOfferView struct {
	// PriceUZS is the monthly price in so'm.
	PriceUZS int    `json:"price_uzs" example:"16000"`
	Period   string `json:"period" example:"month" enums:"month"`
	// Plan is the caller's plan now.
	Plan string `json:"plan" example:"free" enums:"free,pro"`
	// ProUntil is when Birga Pro given by date ends; null when never given.
	ProUntil *time.Time `json:"pro_until"`
}

// GetProOffer godoc swagger
// @Summary the Birga Pro offer: price and the caller's plan
// @Description There is no payment endpoint yet; an admin gives Pro with PUT /v1/admin/users/{id}/pro.
// @Tags pro
// @Security BearerAuth
// @Produce json
// @Success 200 {object} rest.R{data=rest.proOfferView}
// @Failure 401 {object} rest.UnauthorizedResponse
// @Failure 500 {object} rest.InternalServerErrorResponse
// @Router /v1/pro [GET]
func (s *Server) GetProOffer() gin.HandlerFunc {
	return func(c *gin.Context) {
		o, err := s.pro.Offer(c.Request.Context(), currentPrincipal(c))
		if err != nil {
			Return(c, nil, err)

			return
		}

		Return(c, proOfferView{PriceUZS: o.PriceUZS, Period: "month", Plan: o.Plan, ProUntil: o.ProUntil}, nil)
	}
}

// SetProRequest is the body of PUT /v1/admin/users/{id}/pro.
type SetProRequest struct {
	// Until is when Pro ends (RFC 3339); null ends it now.
	Until *time.Time `json:"until" example:"2026-11-25T00:00:00+05:00"`
}

// SetUserPro godoc swagger
// @Summary gives Birga Pro to a user until a date, or ends it
// @Description Sets users.pro_until. The user is Pro while it is in the future; {"until": null} ends Pro.
// @Description It takes effect on the user's next request. Roles paid_user and admin are always Pro.
// @Tags admin
// @Security AdminKey
// @Accept json
// @Produce json
// @Param id path string true "user id (UUID)"
// @Param body body SetProRequest true "end of Pro"
// @Success 200 {object} rest.R{data=rest.userView}
// @Failure 400 {object} rest.BadRequestResponse
// @Failure 401 {object} rest.UnauthorizedResponse
// @Failure 404 {object} rest.NotFoundResponse
// @Failure 500 {object} rest.InternalServerErrorResponse
// @Router /v1/admin/users/{id}/pro [PUT]
func (s *Server) SetUserPro() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := ValidateUUID(id); err != nil {
			Return(c, nil, err)

			return
		}

		var req SetProRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Return(c, nil, errs.Errf(errs.ErrBadRequest, "invalid JSON body: %s", err.Error()))

			return
		}

		u, err := s.pro.Grant(c.Request.Context(), id, req.Until)
		if err != nil {
			Return(c, nil, err)

			return
		}

		Return(c, toUserView(u), nil)
	}
}
