package rest

import (
	"time"

	"github.com/gin-gonic/gin"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
	todaylist "gitlab.com/loyihalar/birga/backend/internal/usecases/today_list"
)

// GetToday godoc swagger
// @Summary returns a child's list for today
// @Description "Bugungi reja": the plan activity of the day and the activities a Pro parent added.
// @Description - plan is the open plan day (status today) or the plan day done today (status done);
// @Description   null when neither exists (the plan is done, or no activity fits the child's age)
// @Description - extras are the added activities, oldest first, each with done
// @Description - done_count counts the done items of the list
// @Description - only the child's parents can ask (404 otherwise)
// @Tags children
// @Security BearerAuth
// @Produce json
// @Param id path string true "child id (UUID)"
// @Success 200 {object} rest.R{data=rest.todayView}
// @Failure 400 {object} rest.BadRequestResponse
// @Failure 401 {object} rest.UnauthorizedResponse
// @Failure 404 {object} rest.NotFoundResponse
// @Failure 500 {object} rest.InternalServerErrorResponse
// @Router /v1/children/{id}/today [GET]
func (s *Server) GetToday() gin.HandlerFunc {
	return func(c *gin.Context) {
		childID := c.Param("id")
		if err := ValidateUUID(childID); err != nil {
			Return(c, nil, err)

			return
		}

		t, err := s.today.Execute(c.Request.Context(), currentUserID(c), childID)
		if err != nil {
			Return(c, nil, err)

			return
		}

		Return(c, toTodayView(t), nil)
	}
}

// AddTodayExtraRequest names the activity to add: goal_id lets the backend pick one, activity_id
// adds that activity. Send exactly one.
type AddTodayExtraRequest struct {
	GoalID     string `json:"goal_id" example:"2b6f0cc9-0f3e-4b1a-9a7e-5d8c3e2f1a00"`
	ActivityID string `json:"activity_id" example:"7b0c1f1e-2d7a-4d8e-9a55-0f4a0d7f9c11"`
}

// AddTodayExtra godoc swagger
// @Summary adds an activity to a child's list for today (Birga Pro)
// @Description - goal_id: picks a published activity for the child's age that serves the goal and is
// @Description   not in today's list yet, never-done ones first (404 when none is left)
// @Description - activity_id: adds that published activity (404 for an unknown or unpublished one)
// @Description - adding an activity already in the list returns it; at most 10 a day (422)
// @Description - mark it done with POST /v1/children/{id}/completions; it does not count toward the streak
// @Description - 403 without Birga Pro
// @Tags children
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "child id (UUID)"
// @Param body body AddTodayExtraRequest true "goal_id or activity_id"
// @Success 200 {object} rest.R{data=rest.todayExtraView}
// @Failure 400 {object} rest.BadRequestResponse
// @Failure 401 {object} rest.UnauthorizedResponse
// @Failure 403 {object} rest.ForbiddenResponse
// @Failure 404 {object} rest.NotFoundResponse
// @Failure 422 {object} rest.UnprocessableContentResponse
// @Failure 500 {object} rest.InternalServerErrorResponse
// @Router /v1/children/{id}/today/extras [POST]
func (s *Server) AddTodayExtra() gin.HandlerFunc {
	return func(c *gin.Context) {
		childID := c.Param("id")
		if err := ValidateUUID(childID); err != nil {
			Return(c, nil, err)

			return
		}

		var req AddTodayExtraRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Return(c, nil, errs.Errf(errs.ErrBadRequest, "invalid JSON body: %s", err.Error()))

			return
		}

		e, err := s.today.AddExtra(c.Request.Context(), todaylist.ExtraRequest{
			UserID: currentUserID(c), Pro: currentUserPro(c), ChildID: childID, GoalID: req.GoalID, ActivityID: req.ActivityID,
		})
		if err != nil {
			Return(c, nil, err)

			return
		}

		Return(c, toTodayExtraView(e), nil)
	}
}

// GetSimilarActivities godoc swagger
// @Summary lists activities similar to one (Birga Pro)
// @Description "O‘xshash mashg‘ulotlar": published activities sharing a goal with this one. With child_id
// @Description they fit the child's age and the ones the child never did come first; without it they
// @Description overlap the activity's age range. 403 without Birga Pro.
// @Tags activities
// @Security BearerAuth
// @Produce json
// @Param id path string true "activity id (UUID)"
// @Param child_id query string false "one of the caller's children"
// @Param limit query int false "how many (default 3, max 10)"
// @Success 200 {object} rest.R{data=[]rest.activityView}
// @Failure 400 {object} rest.BadRequestResponse
// @Failure 401 {object} rest.UnauthorizedResponse
// @Failure 403 {object} rest.ForbiddenResponse
// @Failure 404 {object} rest.NotFoundResponse
// @Failure 422 {object} rest.UnprocessableContentResponse
// @Failure 500 {object} rest.InternalServerErrorResponse
// @Router /v1/activities/{id}/similar [GET]
func (s *Server) GetSimilarActivities() gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, err := parseOptionalInt(c, "limit")
		if err != nil {
			Return(c, nil, err)

			return
		}

		items, err := s.today.Similar(c.Request.Context(), currentUserID(c), currentUserPro(c), c.Param("id"), c.Query("child_id"), limit)
		if err != nil {
			Return(c, nil, err)

			return
		}

		out := make([]activityView, 0, len(items))
		for i := range items {
			out = append(out, toActivityView(items[i]))
		}

		Return(c, out, nil)
	}
}

type todayPlanView struct {
	PlanDay  int          `json:"plan_day" example:"3"`
	Status   string       `json:"status" example:"today" enums:"today,done"`
	Done     bool         `json:"done" example:"false"`
	Activity activityView `json:"activity"`
}

type todayExtraView struct {
	Activity activityView `json:"activity"`
	Done     bool         `json:"done" example:"true"`
	AddedAt  time.Time    `json:"added_at"`
}

type todayView struct {
	Day       string           `json:"day" example:"2026-10-25"`
	Plan      *todayPlanView   `json:"plan"`
	Extras    []todayExtraView `json:"extras"`
	DoneCount int              `json:"done_count" example:"2"`
}

func toTodayExtraView(e domain.DayExtra) todayExtraView {
	return todayExtraView{Activity: toActivityView(e.Activity), Done: e.Done, AddedAt: e.AddedAt}
}

func toTodayView(t domain.Today) todayView {
	v := todayView{Day: t.Day.Format(time.DateOnly), Extras: make([]todayExtraView, 0, len(t.Extras)), DoneCount: t.DoneCount}
	if t.Plan != nil {
		v.Plan = &todayPlanView{
			PlanDay: t.Plan.Day, Status: t.Plan.Status, Done: t.Plan.Status == domain.PlanDayDone, Activity: toActivityView(t.Plan.Activity),
		}
	}

	for i := range t.Extras {
		v.Extras = append(v.Extras, toTodayExtraView(t.Extras[i]))
	}

	return v
}
