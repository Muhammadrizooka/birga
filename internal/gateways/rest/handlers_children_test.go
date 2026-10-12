package rest

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
	activityrecommender "gitlab.com/loyihalar/birga/backend/internal/usecases/activity_recommender"
	completionrecorder "gitlab.com/loyihalar/birga/backend/internal/usecases/completion_recorder"
)

const (
	testUserID  = "9a8b7c6d-5e4f-4a3b-2c1d-0e9f8a7b6c5d"
	testChildID = "3f1d2c4b-8a9e-4b7c-9d2e-1a2b3c4d5e6f"
)

var bearer = map[string]string{authorizationHeader: "Bearer good"}

type fakeChildActivities struct {
	gotCreate    domain.Child
	gotUpdate    domain.ChildUpdate
	gotRestart   bool
	gotRecommend activityrecommender.Request
	gotRecord    completionrecorder.Request
	gotList      domain.CompletionFilter
	gotUserID    string
	err          error
}

func (f *fakeChildActivities) Execute(_ context.Context, req activityrecommender.Request) (domain.Activity, error) {
	f.gotRecommend = req

	return domain.Activity{ID: testID}, f.err
}

func (f *fakeChildActivities) record(req completionrecorder.Request) (domain.Completion, error) {
	f.gotRecord = req

	return domain.Completion{ID: testID, ChildID: req.ChildID, ActivityID: req.ActivityID,
		CompletedOn: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC), Note: req.Note}, f.err
}

func (f *fakeChildActivities) list(userID string, fl domain.CompletionFilter) ([]domain.Completion, int, error) {
	f.gotUserID, f.gotList = userID, fl

	return []domain.Completion{{ID: testID}}, 1, f.err
}

func (f *fakeChildActivities) streak(userID, childID string) (domain.Streak, error) {
	f.gotUserID = userID
	last := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)

	return domain.Streak{Current: 2, Longest: 5, CompletedToday: true, ThisWeek: 2, Total: 9, LastCompletedOn: &last}, f.err
}

func (f *fakeChildActivities) create(userID string, c domain.Child) (domain.Child, error) {
	f.gotUserID, f.gotCreate = userID, c
	c.ID = testChildID

	return c, f.err
}

func (f *fakeChildActivities) update(userID, childID string, upd domain.ChildUpdate) (domain.Child, error) {
	f.gotUserID, f.gotUpdate = userID, upd

	return domain.Child{ID: childID, Name: "Amir", Age: 5, Gender: "male"}, f.err
}

func (f *fakeChildActivities) plan(userID, childID string, restart bool) (domain.Plan, error) {
	f.gotUserID, f.gotRestart = userID, restart
	done := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)

	return domain.Plan{
		ChildID:   childID,
		StartedOn: done,
		Days: []domain.PlanDay{
			{PlanItem: domain.PlanItem{Theme: domain.PlanTheme{Day: 1, TitleUz: "Hisni top", Icon: "heart"}, Activity: domain.Activity{ID: testID}},
				Status: domain.PlanDayDone, CompletedOn: &done},
			{PlanItem: domain.PlanItem{Theme: domain.PlanTheme{Day: 2, TitleUz: "Meni tingla", Icon: "chat"}, Activity: domain.Activity{ID: testID}},
				Status: domain.PlanDayToday},
		},
	}, f.err
}

type childUpdaterFunc func(string, string, domain.ChildUpdate) (domain.Child, error)

func (fn childUpdaterFunc) Execute(_ context.Context, userID, childID string, upd domain.ChildUpdate) (domain.Child, error) {
	return fn(userID, childID, upd)
}

type planGetterFunc func(string, string, bool) (domain.Plan, error)

func (fn planGetterFunc) Execute(_ context.Context, userID, childID string, restart bool) (domain.Plan, error) {
	return fn(userID, childID, restart)
}

type childCreatorFunc func(string, domain.Child) (domain.Child, error)

func (fn childCreatorFunc) Execute(_ context.Context, userID string, c domain.Child) (domain.Child, error) {
	return fn(userID, c)
}

type recorderFunc func(completionrecorder.Request) (domain.Completion, error)

func (fn recorderFunc) Execute(_ context.Context, req completionrecorder.Request) (domain.Completion, error) {
	return fn(req)
}

type completionListerFunc func(string, domain.CompletionFilter) ([]domain.Completion, int, error)

func (fn completionListerFunc) Execute(_ context.Context, userID string, f domain.CompletionFilter) ([]domain.Completion, int, error) {
	return fn(userID, f)
}

type streakFunc func(string, string) (domain.Streak, error)

func (fn streakFunc) Execute(_ context.Context, userID, childID string) (domain.Streak, error) {
	return fn(userID, childID)
}

func TestUserAuth(t *testing.T) {
	s, _ := newTestServer("")
	path := "/v1/children/" + testChildID + "/streak"

	for name, h := range map[string]map[string]string{
		"missing": nil,
		"basic":   {authorizationHeader: "Basic abc"},
		"invalid": {authorizationHeader: "Bearer bad"},
	} {
		if code, r := do(t, s, http.MethodGet, path, "", h); code != http.StatusUnauthorized || r.ErrorCode != _errCodeUnauthorized {
			t.Fatalf("%s: got %d %+v", name, code, r)
		}
	}
}

func TestRecommendActivity(t *testing.T) {
	s, d := newTestServer("")

	code, r := do(t, s, http.MethodGet, "/v1/children/"+testChildID+"/recommendation?goal_id="+testID+"&minutes=10", "", bearer)
	if code != http.StatusOK {
		t.Fatalf("got %d %+v", code, r)
	}

	want := activityrecommender.Request{UserID: testUserID, ChildID: testChildID, GoalID: testID, MaxMinutes: 10}
	if d.kids.gotRecommend != want {
		t.Fatalf("request = %+v, want %+v", d.kids.gotRecommend, want)
	}

	if code, _ := do(t, s, http.MethodGet, "/v1/children/"+testChildID+"/recommendation?minutes=ten", "", bearer); code != http.StatusBadRequest {
		t.Fatalf("bad minutes: %d", code)
	}

	if code, _ := do(t, s, http.MethodGet, "/v1/children/nope/recommendation", "", bearer); code != http.StatusBadRequest {
		t.Fatalf("bad child id: %d", code)
	}

	d.kids.err = errs.ErrNoRecommendation

	if code, _ := do(t, s, http.MethodGet, "/v1/children/"+testChildID+"/recommendation", "", bearer); code != http.StatusNotFound {
		t.Fatalf("no recommendation: %d", code)
	}
}

func TestCompleteActivity(t *testing.T) {
	s, d := newTestServer("")
	path := "/v1/children/" + testChildID + "/completions"

	code, r := do(t, s, http.MethodPost, path, `{"activity_id":"`+testID+`","note":"zo'r"}`, bearer)
	if code != http.StatusOK {
		t.Fatalf("got %d %+v", code, r)
	}

	got := d.kids.gotRecord
	if got.UserID != testUserID || got.ChildID != testChildID || got.ActivityID != testID || got.Note == nil || *got.Note != "zo'r" {
		t.Fatalf("request not mapped: %+v", got)
	}

	if data, _ := r.Data.(map[string]any); data["completed_on"] != "2026-10-03" {
		t.Fatalf("completed_on = %v", data["completed_on"])
	}

	if code, _ := do(t, s, http.MethodPost, path, `{"activity_id":"x"}`, bearer); code != http.StatusUnprocessableEntity {
		t.Fatalf("bad activity id: %d", code)
	}

	if code, _ := do(t, s, http.MethodPost, path, `{broken`, bearer); code != http.StatusBadRequest {
		t.Fatalf("bad body: %d", code)
	}
}

func TestListCompletionsAndStreak(t *testing.T) {
	s, d := newTestServer("")
	base := "/v1/children/" + testChildID

	if code, r := do(t, s, http.MethodGet, base+"/completions?limit=5", "", bearer); code != http.StatusOK {
		t.Fatalf("list: %d %+v", code, r)
	}

	want := domain.CompletionFilter{ChildID: testChildID, Limit: 5}
	if d.kids.gotList != want || d.kids.gotUserID != testUserID {
		t.Fatalf("filter = %+v user %q", d.kids.gotList, d.kids.gotUserID)
	}

	code, r := do(t, s, http.MethodGet, base+"/streak", "", bearer)
	if code != http.StatusOK {
		t.Fatalf("streak: %d %+v", code, r)
	}

	if data, _ := r.Data.(map[string]any); data["current"] != float64(2) || data["last_completed_on"] != "2026-10-03" {
		t.Fatalf("streak view: %+v", r.Data)
	}

	d.kids.err = errs.ErrChildNotFound

	if code, _ := do(t, s, http.MethodGet, base+"/streak", "", bearer); code != http.StatusNotFound {
		t.Fatalf("not my child: %d", code)
	}
}

func TestCreateChild(t *testing.T) {
	s, d := newTestServer("")

	code, r := do(t, s, http.MethodPost, "/v1/children", `{"name":"Amir","age":4,"gender":"male"}`, bearer)
	if code != http.StatusOK {
		t.Fatalf("got %d %+v", code, r)
	}

	want := domain.Child{Name: "Amir", Age: 4, Gender: domain.GenderMale}
	if d.kids.gotUserID != testUserID || d.kids.gotCreate != want {
		t.Fatalf("user %q child %+v", d.kids.gotUserID, d.kids.gotCreate)
	}

	if data, _ := r.Data.(map[string]any); data["id"] != testChildID {
		t.Fatalf("data = %+v", r.Data)
	}

	if code, r := do(t, s, http.MethodPost, "/v1/children", `{"name":"Amir","gender":"male"}`, bearer); code != http.StatusUnprocessableEntity {
		t.Fatalf("missing age: %d %+v", code, r)
	}

	if code, _ := do(t, s, http.MethodPost, "/v1/children", `{"name":"Amir","age":4,"gender":"male"}`, nil); code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", code)
	}

	d.kids.err = errs.Errf(errs.ErrValidation, "gender must be male or female")

	if code, _ := do(t, s, http.MethodPost, "/v1/children", `{"name":"Amir","age":4,"gender":"x"}`, bearer); code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid: %d", code)
	}
}

func TestUserAuth_Roles(t *testing.T) {
	s, _ := newTestServer("")
	s.router.GET("/test/admin-only", s.userAuth(domain.UserRoleAdmin), func(c *gin.Context) { Return(c, currentUserID(c), nil) })

	if code, r := do(t, s, http.MethodGet, "/test/admin-only", "", bearer); code != http.StatusForbidden || r.ErrorCode != _errCodeForbidden {
		t.Fatalf("user on admin route: %d %+v", code, r)
	}

	if code, r := do(t, s, http.MethodGet, "/test/admin-only", "", map[string]string{authorizationHeader: "Bearer admin"}); code != http.StatusOK || r.Data != testUserID {
		t.Fatalf("admin: %d %+v", code, r)
	}
}

func TestAdminAuth_BearerToken(t *testing.T) {
	s, _ := newTestServer(testAdminKey)

	if code, _ := do(t, s, http.MethodGet, "/v1/admin/users", "", map[string]string{authorizationHeader: "Bearer admin"}); code != http.StatusOK {
		t.Fatalf("admin token: %d", code)
	}

	// A bearer token decides on its own; a valid key does not rescue a non-admin token.
	h := map[string]string{authorizationHeader: "Bearer good", adminKeyHeader: testAdminKey}
	if code, _ := do(t, s, http.MethodGet, "/v1/admin/users", "", h); code != http.StatusForbidden {
		t.Fatalf("user token: %d", code)
	}

	if code, _ := do(t, s, http.MethodGet, "/v1/admin/users", "", map[string]string{adminKeyHeader: testAdminKey}); code != http.StatusOK {
		t.Fatalf("admin key: %d", code)
	}
}

func TestUpdateChild(t *testing.T) {
	s, d := newTestServer("")

	code, r := do(t, s, http.MethodPatch, "/v1/children/"+testChildID, `{"age": 5, "photo_id": ""}`, bearer)
	if code != http.StatusOK || d.kids.gotUserID != testUserID {
		t.Fatalf("got %d %+v", code, r)
	}

	if u := d.kids.gotUpdate; u.Age == nil || *u.Age != 5 || u.PhotoID == nil || *u.PhotoID != "" || u.Name != nil || u.Gender != nil {
		t.Fatalf("unexpected update: %+v", u)
	}

	if code, _ := do(t, s, http.MethodPatch, "/v1/children/"+testChildID, `{`, bearer); code != http.StatusBadRequest {
		t.Fatalf("bad json: %d", code)
	}

	if code, _ := do(t, s, http.MethodPatch, "/v1/children/nope", `{}`, bearer); code != http.StatusBadRequest {
		t.Fatalf("bad id: %d", code)
	}

	if code, _ := do(t, s, http.MethodPatch, "/v1/children/"+testChildID, `{"age": 5}`, nil); code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", code)
	}

	d.kids.err = errs.ErrChildNotFound

	if code, _ := do(t, s, http.MethodPatch, "/v1/children/"+testChildID, `{"age": 5}`, bearer); code != http.StatusNotFound {
		t.Fatalf("not found: %d", code)
	}
}

func TestPlan(t *testing.T) {
	s, d := newTestServer("")

	code, r := do(t, s, http.MethodGet, "/v1/children/"+testChildID+"/plan", "", bearer)
	if code != http.StatusOK || d.kids.gotRestart {
		t.Fatalf("get: %d %+v restart=%v", code, r, d.kids.gotRestart)
	}

	data, _ := r.Data.(map[string]any)
	days, _ := data["days"].([]any)

	if data["started_on"] != "2026-10-10" || len(days) != 2 {
		t.Fatalf("unexpected plan: %+v", r.Data)
	}

	first, _ := days[0].(map[string]any)
	second, _ := days[1].(map[string]any)

	if first["status"] != "done" || first["completed_on"] != "2026-10-10" || first["icon"] != "heart" ||
		second["status"] != "today" || second["completed_on"] != nil {
		t.Fatalf("unexpected days: %+v", days)
	}

	if code, _ := do(t, s, http.MethodPost, "/v1/children/"+testChildID+"/plan", "", bearer); code != http.StatusOK || !d.kids.gotRestart {
		t.Fatalf("restart: %d restart=%v", code, d.kids.gotRestart)
	}

	d.kids.err = errs.ErrNoRecommendation

	if code, _ := do(t, s, http.MethodGet, "/v1/children/"+testChildID+"/plan", "", bearer); code != http.StatusNotFound {
		t.Fatalf("empty catalogue: %d", code)
	}
}
