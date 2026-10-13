package rest

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
	completionrecorder "gitlab.com/loyihalar/birga/backend/internal/usecases/completion_recorder"
)

const (
	testUserID  = "9a8b7c6d-5e4f-4a3b-2c1d-0e9f8a7b6c5d"
	testChildID = "3f1d2c4b-8a9e-4b7c-9d2e-1a2b3c4d5e6f"
)

var bearer = map[string]string{authorizationHeader: "Bearer good"}

type fakeChildActivities struct {
	gotCreate  domain.Child
	gotUpdate  domain.ChildUpdate
	gotRestart bool
	gotRecord  completionrecorder.Request
	gotList    domain.CompletionFilter
	gotRange   [2]time.Time
	gotUserID  string
	todayDone  bool
	err        error
}

func (f *fakeChildActivities) record(req completionrecorder.Request) (domain.Completion, error) {
	f.gotRecord = req

	day := 2

	return domain.Completion{ID: testID, ChildID: req.ChildID, ActivityID: req.ActivityID,
		CompletedOn: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC), Note: req.Note, PlanDay: &day}, f.err
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

func (f *fakeChildActivities) streakDays(userID, childID string, from, to time.Time) ([]domain.StreakDay, error) {
	f.gotUserID, f.gotRange = userID, [2]time.Time{from, to}

	return []domain.StreakDay{
		{Day: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), Status: domain.StreakDayMissed},
		{Day: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC), Status: domain.StreakDayDone},
	}, f.err
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

	goal := testID
	second := domain.PlanDayToday

	if f.todayDone {
		second = domain.PlanDayLocked
	}

	return domain.Plan{
		ID:        "p1",
		ChildID:   childID,
		Number:    2,
		StartedOn: done,
		Days: []domain.PlanDay{
			{PlanItem: domain.PlanItem{Day: 1, GoalID: &goal, Activity: domain.Activity{ID: "a1"}, CompletedOn: &done},
				Status: domain.PlanDayDone},
			{PlanItem: domain.PlanItem{Day: 2, Activity: domain.Activity{ID: "a2"}}, Status: second},
		},
	}, f.err
}

type childUpdaterFunc func(string, string, domain.ChildUpdate) (domain.Child, error)

func (fn childUpdaterFunc) Execute(_ context.Context, userID, childID string, upd domain.ChildUpdate) (domain.Child, error) {
	return fn(userID, childID, upd)
}

type planGetterFunc func(string, string, bool) (domain.Plan, error)

func (fn planGetterFunc) Execute(_ context.Context, userID, childID string) (domain.Plan, error) {
	return fn(userID, childID, false)
}

func (fn planGetterFunc) Next(_ context.Context, userID, childID string) (domain.Plan, error) {
	return fn(userID, childID, true)
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

type streakGetterFake struct{ f *fakeChildActivities }

func (s streakGetterFake) Execute(_ context.Context, userID, childID string) (domain.Streak, error) {
	return s.f.streak(userID, childID)
}

func (s streakGetterFake) Days(_ context.Context, userID, childID string, from, to time.Time) ([]domain.StreakDay, error) {
	return s.f.streakDays(userID, childID, from, to)
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
	path := "/v1/children/" + testChildID + "/recommendation"

	code, r := do(t, s, http.MethodGet, path, "", bearer)
	if data, _ := r.Data.(map[string]any); code != http.StatusOK || data["id"] != "a2" || d.kids.gotRestart {
		t.Fatalf("got %d %+v", code, r)
	}

	if code, _ := do(t, s, http.MethodGet, "/v1/children/nope/recommendation", "", bearer); code != http.StatusBadRequest {
		t.Fatalf("bad child id: %d", code)
	}

	d.kids.todayDone = true

	if code, r := do(t, s, http.MethodGet, path, "", bearer); code != http.StatusNotFound || r.ErrorNote != errs.ErrNoPlanDayToday.Error() {
		t.Fatalf("nothing open today: %d %+v", code, r)
	}

	d.kids.err = errs.ErrNoRecommendation

	if code, _ := do(t, s, http.MethodGet, path, "", bearer); code != http.StatusNotFound {
		t.Fatalf("no activity: %d", code)
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

	if data, _ := r.Data.(map[string]any); data["completed_on"] != "2026-10-03" || data["plan_day"] != float64(2) {
		t.Fatalf("data = %v", data)
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

	code, r = do(t, s, http.MethodGet, base+"/streak/days?from=2026-10-02&to=2026-10-03", "", bearer)
	days, _ := r.Data.([]any)

	if code != http.StatusOK || len(days) != 2 || !d.kids.gotRange[0].Equal(time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("streak days: %d %+v %v", code, r, d.kids.gotRange)
	}

	if first, _ := days[0].(map[string]any); first["day"] != "2026-10-02" || first["status"] != "missed" {
		t.Fatalf("first day: %+v", first)
	}

	if code, _ := do(t, s, http.MethodGet, base+"/streak/days", "", bearer); code != http.StatusOK || !d.kids.gotRange[0].IsZero() {
		t.Fatalf("default range: %d %v", code, d.kids.gotRange)
	}

	if code, _ := do(t, s, http.MethodGet, base+"/streak/days?from=yesterday", "", bearer); code != http.StatusUnprocessableEntity {
		t.Fatalf("bad date: %d", code)
	}

	d.kids.err = errs.ErrChildNotFound

	if code, _ := do(t, s, http.MethodGet, base+"/streak", "", bearer); code != http.StatusNotFound {
		t.Fatalf("not my child: %d", code)
	}
}

func TestCreateChild(t *testing.T) {
	s, d := newTestServer("")

	code, r := do(t, s, http.MethodPost, "/v1/children", `{"name":"Amir","age":4,"gender":"male","goal_ids":["`+testID+`"]}`, bearer)
	if code != http.StatusOK {
		t.Fatalf("got %d %+v", code, r)
	}

	got := d.kids.gotCreate
	if d.kids.gotUserID != testUserID || got.Name != "Amir" || got.Age != 4 || got.Gender != domain.GenderMale ||
		len(got.GoalIDs) != 1 || got.GoalIDs[0] != testID {
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

	code, r := do(t, s, http.MethodPatch, "/v1/children/"+testChildID, `{"age": 5, "photo_id": "", "goal_ids": ["`+testID+`"]}`, bearer)
	if code != http.StatusOK || d.kids.gotUserID != testUserID {
		t.Fatalf("got %d %+v", code, r)
	}

	if u := d.kids.gotUpdate; u.Age == nil || *u.Age != 5 || u.PhotoID == nil || *u.PhotoID != "" || u.Name != nil || u.Gender != nil ||
		u.GoalIDs == nil || len(*u.GoalIDs) != 1 {
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

	if data["number"] != float64(2) || first["status"] != "done" || first["completed_on"] != "2026-10-10" || first["goal_id"] != testID ||
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

func TestPlan_NotDone(t *testing.T) {
	s, d := newTestServer("")
	d.kids.err = errs.ErrPlanNotDone

	if code, r := do(t, s, http.MethodPost, "/v1/children/"+testChildID+"/plan", "", bearer); code != http.StatusConflict {
		t.Fatalf("got %d %+v", code, r)
	}
}
