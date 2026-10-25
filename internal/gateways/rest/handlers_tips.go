package rest

import (
	"time"

	"github.com/gin-gonic/gin"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

// TopicRequest is the body of POST /v1/admin/topics and PATCH /v1/admin/topics/{id}. On PATCH,
// omitted or null names are left unchanged.
type TopicRequest struct {
	NameUz *string `json:"name_uz" example:"Uyqu"`
	NameRu *string `json:"name_ru" example:"Сон"`
	NameEn *string `json:"name_en" example:"Sleep"`
}

// ListTopics godoc swagger
// @Summary lists the active topics (tags of tips, articles and videos), oldest first
// @Description Open without a token. Not paginated: the list is short.
// @Tags topics
// @Produce json
// @Success 200 {object} rest.R{data=[]rest.topicView}
// @Failure 500 {object} rest.InternalServerErrorResponse
// @Router /v1/topics [GET]
func (s *Server) ListTopics() gin.HandlerFunc {
	return func(c *gin.Context) {
		items, err := s.topics.List(c.Request.Context())
		if err != nil {
			Return(c, nil, err)

			return
		}

		views := make([]topicView, 0, len(items))
		for i := range items {
			views = append(views, toTopicView(items[i]))
		}

		Return(c, views, nil)
	}
}

// CreateTopic godoc swagger
// @Summary creates a topic
// @Description name_uz, name_ru and name_en are required, at most 100 characters, each unique among active topics (409).
// @Tags admin
// @Security AdminKey
// @Accept json
// @Produce json
// @Param body body TopicRequest true "topic"
// @Success 200 {object} rest.R{data=rest.topicView}
// @Failure 400 {object} rest.BadRequestResponse
// @Failure 401 {object} rest.UnauthorizedResponse
// @Failure 409 {object} rest.ConflictResponse
// @Failure 422 {object} rest.UnprocessableContentResponse
// @Failure 500 {object} rest.InternalServerErrorResponse
// @Router /v1/admin/topics [POST]
func (s *Server) CreateTopic() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req TopicRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Return(c, nil, errs.Errf(errs.ErrBadRequest, "invalid JSON body: %s", err.Error()))

			return
		}

		t, err := s.topics.Create(c.Request.Context(), domain.Topic{
			NameUz: deref(req.NameUz), NameRu: deref(req.NameRu), NameEn: deref(req.NameEn),
		})
		if err != nil {
			Return(c, nil, err)

			return
		}

		Return(c, toTopicView(t), nil)
	}
}

// UpdateTopic godoc swagger
// @Summary renames a topic
// @Description Only the names present change; a given name cannot be empty.
// @Tags admin
// @Security AdminKey
// @Accept json
// @Produce json
// @Param id path string true "topic id (UUID)"
// @Param body body TopicRequest true "names to change"
// @Success 200 {object} rest.R{data=rest.topicView}
// @Failure 400 {object} rest.BadRequestResponse
// @Failure 401 {object} rest.UnauthorizedResponse
// @Failure 404 {object} rest.NotFoundResponse
// @Failure 409 {object} rest.ConflictResponse
// @Failure 422 {object} rest.UnprocessableContentResponse
// @Failure 500 {object} rest.InternalServerErrorResponse
// @Router /v1/admin/topics/{id} [PATCH]
func (s *Server) UpdateTopic() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := ValidateUUID(id); err != nil {
			Return(c, nil, err)

			return
		}

		var req TopicRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Return(c, nil, errs.Errf(errs.ErrBadRequest, "invalid JSON body: %s", err.Error()))

			return
		}

		t, err := s.topics.Update(c.Request.Context(), id, domain.TopicUpdate{NameUz: req.NameUz, NameRu: req.NameRu, NameEn: req.NameEn})
		if err != nil {
			Return(c, nil, err)

			return
		}

		Return(c, toTopicView(t), nil)
	}
}

// DeleteTopic godoc swagger
// @Summary soft-deletes a topic
// @Description It leaves GET /v1/topics; tips, articles and videos tagged with it keep the tag id.
// @Tags admin
// @Security AdminKey
// @Produce json
// @Param id path string true "topic id (UUID)"
// @Success 200 {object} rest.R
// @Failure 400 {object} rest.BadRequestResponse
// @Failure 401 {object} rest.UnauthorizedResponse
// @Failure 404 {object} rest.NotFoundResponse
// @Failure 500 {object} rest.InternalServerErrorResponse
// @Router /v1/admin/topics/{id} [DELETE]
func (s *Server) DeleteTopic() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := ValidateUUID(id); err != nil {
			Return(c, nil, err)

			return
		}

		Return(c, nil, s.topics.Delete(c.Request.Context(), id))
	}
}

// CreateTipRequest is the body of POST /v1/admin/tips.
type CreateTipRequest struct {
	TextUz  string `json:"text_uz" example:"Ovqat paytida telefonni chetga qo‘ying."`
	TextRu  string `json:"text_ru" example:"Во время еды отложите телефон."`
	TextEn  string `json:"text_en" example:"Put the phone away at mealtimes."`
	TopicID string `json:"topic_id" example:"9a1c7e52-3b4d-4f6a-8e2b-7c5d1f0a9b3e"`
	MinAge  int    `json:"min_age" example:"2" minimum:"2" maximum:"6"`
	MaxAge  int    `json:"max_age" example:"6" minimum:"2" maximum:"6"`
	Status  string `json:"status" example:"in_review" enums:"published,in_review,unpublished"`
}

// CreateTip godoc swagger
// @Summary creates a tip ("Kun maslahati")
// @Description - text_uz, text_ru and text_en are required, at most 500 characters each
// @Description - topic_id is an active topic from GET /v1/topics (422 if unknown)
// @Description - min_age/max_age are child ages within 2..6
// @Description - status is published, in_review or unpublished; omitted means in_review. Only published tips reach the app
// @Tags admin
// @Security AdminKey
// @Accept json
// @Produce json
// @Param body body CreateTipRequest true "tip"
// @Success 200 {object} rest.R{data=rest.tipView}
// @Failure 400 {object} rest.BadRequestResponse
// @Failure 401 {object} rest.UnauthorizedResponse
// @Failure 422 {object} rest.UnprocessableContentResponse
// @Failure 500 {object} rest.InternalServerErrorResponse
// @Router /v1/admin/tips [POST]
func (s *Server) CreateTip() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateTipRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Return(c, nil, errs.Errf(errs.ErrBadRequest, "invalid JSON body: %s", err.Error()))

			return
		}

		t, err := s.tips.Create(c.Request.Context(), domain.Tip{
			TextUz: req.TextUz, TextRu: req.TextRu, TextEn: req.TextEn, TopicID: req.TopicID,
			MinAge: req.MinAge, MaxAge: req.MaxAge, Status: req.Status,
		})
		if err != nil {
			Return(c, nil, err)

			return
		}

		Return(c, toTipView(t), nil)
	}
}

// AdminListTips godoc swagger
// @Summary lists tips in every status
// @Tags admin
// @Security AdminKey
// @Produce json
// @Param q query string false "text the tip (any language) contains, ignoring case (at most 100 characters)"
// @Param status query string false "published, in_review or unpublished; omitted lists all" Enums(published, in_review, unpublished)
// @Param topic_id query string false "topic id (UUID) from GET /v1/topics"
// @Param age query int false "child age in years (2..6)"
// @Param limit query int false "page size (default 20, max 100)"
// @Param offset query int false "offset"
// @Success 200 {object} rest.R{data=rest.tipListView}
// @Failure 400 {object} rest.BadRequestResponse
// @Failure 401 {object} rest.UnauthorizedResponse
// @Failure 422 {object} rest.UnprocessableContentResponse
// @Failure 500 {object} rest.InternalServerErrorResponse
// @Router /v1/admin/tips [GET]
func (s *Server) AdminListTips() gin.HandlerFunc {
	return func(c *gin.Context) {
		f, ok := tipFilter(c)
		if !ok {
			return
		}

		f.Status = c.Query("status")

		s.listTips(c, f)
	}
}

// AdminGetTip godoc swagger
// @Summary returns a tip in any status
// @Tags admin
// @Security AdminKey
// @Produce json
// @Param id path string true "tip id (UUID)"
// @Success 200 {object} rest.R{data=rest.tipView}
// @Failure 400 {object} rest.BadRequestResponse
// @Failure 401 {object} rest.UnauthorizedResponse
// @Failure 404 {object} rest.NotFoundResponse
// @Failure 500 {object} rest.InternalServerErrorResponse
// @Router /v1/admin/tips/{id} [GET]
func (s *Server) AdminGetTip() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := ValidateUUID(id); err != nil {
			Return(c, nil, err)

			return
		}

		t, err := s.tips.Get(c.Request.Context(), id, false)
		if err != nil {
			Return(c, nil, err)

			return
		}

		Return(c, toTipView(t), nil)
	}
}

// UpdateTipRequest is the body of PATCH /v1/admin/tips/{id}. Omitted or null fields are left unchanged.
type UpdateTipRequest struct {
	TextUz  *string `json:"text_uz"`
	TextRu  *string `json:"text_ru"`
	TextEn  *string `json:"text_en"`
	TopicID *string `json:"topic_id" example:"9a1c7e52-3b4d-4f6a-8e2b-7c5d1f0a9b3e"`
	MinAge  *int    `json:"min_age" example:"2" minimum:"2" maximum:"6"`
	MaxAge  *int    `json:"max_age" example:"6" minimum:"2" maximum:"6"`
	Status  *string `json:"status" example:"published" enums:"published,in_review,unpublished"`
}

// UpdateTip godoc swagger
// @Summary updates a tip partially, including its status
// @Description Same rules as create; texts cannot be emptied; the resulting age range must stay valid (422).
// @Tags admin
// @Security AdminKey
// @Accept json
// @Produce json
// @Param id path string true "tip id (UUID)"
// @Param body body UpdateTipRequest true "fields to change"
// @Success 200 {object} rest.R{data=rest.tipView}
// @Failure 400 {object} rest.BadRequestResponse
// @Failure 401 {object} rest.UnauthorizedResponse
// @Failure 404 {object} rest.NotFoundResponse
// @Failure 422 {object} rest.UnprocessableContentResponse
// @Failure 500 {object} rest.InternalServerErrorResponse
// @Router /v1/admin/tips/{id} [PATCH]
func (s *Server) UpdateTip() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := ValidateUUID(id); err != nil {
			Return(c, nil, err)

			return
		}

		var req UpdateTipRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Return(c, nil, errs.Errf(errs.ErrBadRequest, "invalid JSON body: %s", err.Error()))

			return
		}

		t, err := s.tips.Update(c.Request.Context(), id, domain.TipUpdate{
			TextUz: req.TextUz, TextRu: req.TextRu, TextEn: req.TextEn, TopicID: req.TopicID,
			MinAge: req.MinAge, MaxAge: req.MaxAge, Status: req.Status,
		})
		if err != nil {
			Return(c, nil, err)

			return
		}

		Return(c, toTipView(t), nil)
	}
}

// DeleteTip godoc swagger
// @Summary soft-deletes a tip
// @Tags admin
// @Security AdminKey
// @Produce json
// @Param id path string true "tip id (UUID)"
// @Success 200 {object} rest.R
// @Failure 400 {object} rest.BadRequestResponse
// @Failure 401 {object} rest.UnauthorizedResponse
// @Failure 404 {object} rest.NotFoundResponse
// @Failure 500 {object} rest.InternalServerErrorResponse
// @Router /v1/admin/tips/{id} [DELETE]
func (s *Server) DeleteTip() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := ValidateUUID(id); err != nil {
			Return(c, nil, err)

			return
		}

		Return(c, nil, s.tips.Delete(c.Request.Context(), id))
	}
}

// ListTips godoc swagger
// @Summary lists published tips (Kutubxona > Maslahat)
// @Description For a user without Birga Pro, every tip that was never their daily tip has locked=true and empty texts.
// @Tags tips
// @Security BearerAuth
// @Produce json
// @Param q query string false "text the tip (any language) contains, ignoring case (at most 100 characters)"
// @Param topic_id query string false "topic id (UUID) from GET /v1/topics"
// @Param age query int false "child age in years (2..6)"
// @Param limit query int false "page size (default 20, max 100)"
// @Param offset query int false "offset"
// @Success 200 {object} rest.R{data=rest.tipListView}
// @Failure 400 {object} rest.BadRequestResponse
// @Failure 401 {object} rest.UnauthorizedResponse
// @Failure 422 {object} rest.UnprocessableContentResponse
// @Failure 500 {object} rest.InternalServerErrorResponse
// @Router /v1/tips [GET]
func (s *Server) ListTips() gin.HandlerFunc {
	return func(c *gin.Context) {
		f, ok := tipFilter(c)
		if !ok {
			return
		}

		f.Status = domain.ArticleStatusPublished
		if !currentUserPro(c) {
			f.LockFor = currentUserID(c)
		}

		s.listTips(c, f)
	}
}

// GetDailyTip godoc swagger
// @Summary returns the signed-in user's tip of the day ("Kun maslahati")
// @Description The tip is picked once per day (Uzbekistan time) and stays the same all day. It prefers a tip whose
// @Description age range fits one of the user's children, not shown before, then the one shown longest ago, rotating
// @Description through the topics. 404 when no tip is published.
// @Tags tips
// @Security BearerAuth
// @Produce json
// @Success 200 {object} rest.R{data=rest.dailyTipView}
// @Failure 401 {object} rest.UnauthorizedResponse
// @Failure 404 {object} rest.NotFoundResponse
// @Failure 500 {object} rest.InternalServerErrorResponse
// @Router /v1/me/tips/daily [GET]
func (s *Server) GetDailyTip() gin.HandlerFunc {
	return func(c *gin.Context) {
		d, err := s.dailyTip.Execute(c.Request.Context(), currentUserID(c))
		if err != nil {
			Return(c, nil, err)

			return
		}

		Return(c, dailyTipView{Day: d.Day.Format(time.DateOnly), Tip: toTipView(d.Tip)}, nil)
	}
}

func (s *Server) listTips(c *gin.Context, f domain.TipFilter) {
	items, total, err := s.tips.List(c.Request.Context(), f)
	if err != nil {
		Return(c, nil, err)

		return
	}

	views := make([]tipView, 0, len(items))
	for i := range items {
		views = append(views, toTipView(items[i]))
	}

	Return(c, tipListView{Items: views, Total: total, Limit: f.Limit, Offset: f.Offset}, nil)
}

// tipFilter reads the shared list query parameters; on a bad value it answers and returns false.
func tipFilter(c *gin.Context) (domain.TipFilter, bool) {
	f, ok := articleFilter(c)

	return domain.TipFilter{Age: f.Age, TopicID: f.TopicID, Search: f.Search, Limit: f.Limit, Offset: f.Offset}, ok
}

func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

type topicView struct {
	ID        string    `json:"id" example:"9a1c7e52-3b4d-4f6a-8e2b-7c5d1f0a9b3e"`
	NameUz    string    `json:"name_uz" example:"Uyqu"`
	NameRu    string    `json:"name_ru" example:"Сон"`
	NameEn    string    `json:"name_en" example:"Sleep"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type tipView struct {
	ID        string    `json:"id" example:"5c3b2a10-7d8e-4f9a-b1c2-d3e4f5a6b7c8"`
	TextUz    string    `json:"text_uz" example:"Ovqat paytida telefonni chetga qo‘ying."`
	TextRu    string    `json:"text_ru" example:"Во время еды отложите телефон."`
	TextEn    string    `json:"text_en" example:"Put the phone away at mealtimes."`
	TopicID   string    `json:"topic_id" example:"9a1c7e52-3b4d-4f6a-8e2b-7c5d1f0a9b3e"`
	MinAge    int       `json:"min_age" example:"2"`
	MaxAge    int       `json:"max_age" example:"6"`
	Status    string    `json:"status" example:"published" enums:"published,in_review,unpublished"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Locked is true on GET /v1/tips for a user without Birga Pro when the tip was never their
	// daily tip; its texts are then empty.
	Locked bool `json:"locked" example:"false"`
}

type tipListView struct {
	Items  []tipView `json:"items"`
	Total  int       `json:"total" example:"52"`
	Limit  int       `json:"limit" example:"20"`
	Offset int       `json:"offset" example:"0"`
}

type dailyTipView struct {
	// Day is today's date in Uzbekistan time.
	Day string  `json:"day" example:"2026-10-25"`
	Tip tipView `json:"tip"`
}

func toTopicView(t domain.Topic) topicView {
	return topicView{ID: t.ID, NameUz: t.NameUz, NameRu: t.NameRu, NameEn: t.NameEn, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}

func toTipView(t domain.Tip) tipView {
	return tipView{
		ID: t.ID, TextUz: t.TextUz, TextRu: t.TextRu, TextEn: t.TextEn, TopicID: t.TopicID,
		MinAge: t.MinAge, MaxAge: t.MaxAge, Status: t.Status, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
		Locked: t.Locked,
	}
}
