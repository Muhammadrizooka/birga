package rest

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
	todaylist "gitlab.com/loyihalar/birga/backend/internal/usecases/today_list"
)

type fakeToday struct {
	gotExtra   todaylist.ExtraRequest
	gotLevel   domain.DifficultyChange
	gotSimilar struct {
		pro             bool
		activity, child string
		limit           int
	}
}

func (f *fakeToday) Execute(_ context.Context, _, _ string) (domain.Today, error) {
	return domain.Today{
		Day:       time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC),
		Plan:      &domain.TodayPlan{Day: 3, Status: domain.PlanDayDone, Activity: domain.Activity{ID: testID}},
		Extras:    []domain.DayExtra{{Activity: domain.Activity{ID: testID}, Done: true}},
		DoneCount: 2,
	}, nil
}

func (f *fakeToday) AddExtra(_ context.Context, req todaylist.ExtraRequest) (domain.DayExtra, error) {
	f.gotExtra = req
	if !req.Pro {
		return domain.DayExtra{}, errs.ErrNeedsPro
	}

	return domain.DayExtra{Activity: domain.Activity{ID: testID}}, nil
}

func (f *fakeToday) ChangeDifficulty(_ context.Context, req domain.DifficultyChange) (domain.Activity, error) {
	f.gotLevel = req

	return domain.Activity{ID: testID, Difficulty: req.Difficulty}, nil
}

func (f *fakeToday) Similar(_ context.Context, _ string, pro bool, activityID, childID string, limit int) ([]domain.Activity, error) {
	f.gotSimilar.pro, f.gotSimilar.activity, f.gotSimilar.child, f.gotSimilar.limit = pro, activityID, childID, limit

	return []domain.Activity{{ID: testID}}, nil
}

func TestToday(t *testing.T) {
	s, d := newTestServer(testAdminKey)

	code, r := do(t, s, http.MethodGet, "/v1/children/"+testID+"/today", "", bearer)
	data := mustJSON(t, r.Data)
	if code != http.StatusOK || string(data["day"]) != `"2026-10-25"` || string(data["done_count"]) != "2" ||
		!strings.Contains(string(data["plan"]), `"done":true`) || !strings.Contains(string(data["extras"]), `"done":true`) {
		t.Fatalf("today: %d %+v", code, r.Data)
	}

	if code, _ := do(t, s, http.MethodPost, "/v1/children/"+testID+"/today/extras", `{"goal_id":"`+testID+`"}`, bearer); code != http.StatusForbidden {
		t.Errorf("free extra: %d", code)
	}

	if code, _ := do(t, s, http.MethodPost, "/v1/children/"+testID+"/today/extras", `{"activity_id":"`+testID+`"}`, proBearer); code != http.StatusOK ||
		d.today.gotExtra.ActivityID != testID || d.today.gotExtra.ChildID != testID || !d.today.gotExtra.Pro {
		t.Errorf("pro extra: %d %+v", code, d.today.gotExtra)
	}

	code, r = do(t, s, http.MethodPost, "/v1/children/"+testID+"/today/difficulty", `{"activity_id":"`+testID+`","difficulty":"easy"}`, proBearer)
	if code != http.StatusOK || d.today.gotLevel.ActivityID != testID || d.today.gotLevel.Difficulty != "easy" || !d.today.gotLevel.Pro ||
		string(mustJSON(t, r.Data)["difficulty"]) != `"easy"` {
		t.Errorf("difficulty: %d %+v", code, d.today.gotLevel)
	}

	if code, _ := do(t, s, http.MethodGet, "/v1/activities/"+testID+"/similar?child_id="+testID+"&limit=5", "", proBearer); code != http.StatusOK ||
		d.today.gotSimilar.limit != 5 || d.today.gotSimilar.child != testID || !d.today.gotSimilar.pro {
		t.Errorf("similar: %d %+v", code, d.today.gotSimilar)
	}

	if code, _ := do(t, s, http.MethodGet, "/v1/activities/"+testID+"/similar", "", nil); code != http.StatusUnauthorized {
		t.Errorf("similar without token: %d", code)
	}
}
