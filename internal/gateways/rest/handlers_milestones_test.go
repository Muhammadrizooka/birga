package rest

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

type fakeMilestones struct {
	created  domain.Milestone
	updated  domain.MilestoneUpdate
	filter   domain.MilestoneFilter
	age      int
	pro      bool
	marked   string
	unmarked string
	month    string
	child    string
}

type fakeMilestoneManager struct{ f *fakeMilestones }

func (m fakeMilestoneManager) Create(_ context.Context, ms domain.Milestone) (domain.Milestone, error) {
	m.f.created = ms
	ms.ID = testID

	return ms, nil
}

func (m fakeMilestoneManager) Get(_ context.Context, id string) (domain.Milestone, error) {
	return domain.Milestone{ID: id}, nil
}

func (m fakeMilestoneManager) Update(_ context.Context, id string, upd domain.MilestoneUpdate) (domain.Milestone, error) {
	m.f.updated = upd

	return domain.Milestone{ID: id}, nil
}

func (m fakeMilestoneManager) Delete(context.Context, string) error { return nil }

func (m fakeMilestoneManager) List(_ context.Context, fl domain.MilestoneFilter) ([]domain.Milestone, int, error) {
	m.f.filter = fl

	return []domain.Milestone{{ID: testID}}, 1, nil
}

func (f *fakeMilestones) List(_ context.Context, _ string, pro bool, _ string, ageMonths int) (domain.ChildMilestones, error) {
	f.pro, f.age = pro, ageMonths

	return domain.ChildMilestones{AgeMonths: 48, Stages: []int{24, 48}, Achieved: 1, Total: 1, Domains: map[string][]domain.ChildMilestone{
		domain.MilestoneThinking: {{Milestone: domain.Milestone{ID: testID, Domain: domain.MilestoneThinking, AdviceLocked: true}, Achieved: true}},
	}}, nil
}

func (f *fakeMilestones) Get(_ context.Context, _ string, pro bool, id, childID string) (domain.MilestoneDetail, error) {
	f.pro, f.child = pro, childID

	return domain.MilestoneDetail{ChildMilestone: domain.ChildMilestone{Milestone: domain.Milestone{ID: id}},
		Activities: []domain.Activity{{ID: testID}}}, nil
}

func (f *fakeMilestones) Mark(_ context.Context, _, _, milestoneID string) error {
	f.marked = milestoneID

	return nil
}

func (f *fakeMilestones) Unmark(_ context.Context, _, _, milestoneID string) error {
	f.unmarked = milestoneID

	return nil
}

func (f *fakeMilestones) Report(_ context.Context, _ string, pro bool, _, month string) (domain.MonthlyReport, error) {
	f.month = month
	if !pro {
		return domain.MonthlyReport{}, errs.ErrNeedsPro
	}

	return domain.MonthlyReport{Month: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), NewMilestones: 3, Completions: 26}, nil
}

func TestMilestones_Admin(t *testing.T) {
	s, d := newTestServer(testAdminKey)

	body := `{"domain":"thinking","age_months":36,"text_uz":"a","text_ru":"b","text_en":"c","how_to_uz":"1\n2","activity_ids":["` + testID + `"]}`
	if code, r := do(t, s, http.MethodPost, "/v1/admin/milestones", body, adminJSON); code != http.StatusOK {
		t.Fatalf("create: %d %+v", code, r)
	}

	if m := d.stones.created; m.Domain != "thinking" || m.AgeMonths != 36 || m.HowToUz != "1\n2" || len(m.ActivityIDs) != 1 {
		t.Errorf("create mapped wrong: %+v", m)
	}

	if code, _ := do(t, s, http.MethodGet, "/v1/admin/milestones?domain=social&age_months=48&status=published&q=gap", "", adminJSON); code != http.StatusOK ||
		d.stones.filter.Domain != "social" || d.stones.filter.AgeMonths != 48 || d.stones.filter.Status != "published" || d.stones.filter.Search != "gap" {
		t.Errorf("list: %d %+v", code, d.stones.filter)
	}

	if code, _ := do(t, s, http.MethodPatch, "/v1/admin/milestones/"+testID, `{"meaning_en":""}`, adminJSON); code != http.StatusOK ||
		d.stones.updated.MeaningEn == nil || d.stones.updated.TextUz != nil {
		t.Errorf("update: %d %+v", code, d.stones.updated)
	}

	if code, _ := do(t, s, http.MethodGet, "/v1/admin/milestones", "", nil); code != http.StatusUnauthorized {
		t.Errorf("without key: %d", code)
	}
}

func TestMilestones_App(t *testing.T) {
	s, d := newTestServer(testAdminKey)

	code, r := do(t, s, http.MethodGet, "/v1/children/"+testID+"/milestones?age_months=48", "", bearer)
	data := mustJSON(t, r.Data)
	if code != http.StatusOK || d.stones.age != 48 || d.stones.pro || string(data["stages"]) != "[24,48]" ||
		!strings.HasPrefix(string(data["domains"]), `[{"domain":"communication","milestones":[]}`) ||
		!strings.Contains(string(data["domains"]), `"advice_locked":true`) {
		t.Fatalf("list: %d %+v", code, r.Data)
	}

	if code, _ := do(t, s, http.MethodPut, "/v1/children/"+testID+"/milestones/"+testID, "", bearer); code != http.StatusOK || d.stones.marked != testID {
		t.Errorf("mark: %d", code)
	}

	if code, _ := do(t, s, http.MethodDelete, "/v1/children/"+testID+"/milestones/"+testID, "", bearer); code != http.StatusOK || d.stones.unmarked != testID {
		t.Errorf("unmark: %d", code)
	}

	if code, r := do(t, s, http.MethodGet, "/v1/milestones/"+testID+"?child_id="+testID, "", proBearer); code != http.StatusOK ||
		!d.stones.pro || d.stones.child != testID || !strings.Contains(string(mustJSON(t, r.Data)["activities"]), testID) {
		t.Errorf("get: %d %+v", code, r.Data)
	}

	if code, _ := do(t, s, http.MethodGet, "/v1/children/"+testID+"/report", "", bearer); code != http.StatusForbidden {
		t.Errorf("free report: %d", code)
	}

	code, r = do(t, s, http.MethodGet, "/v1/children/"+testID+"/report?month=2026-10", "", proBearer)
	if code != http.StatusOK || d.stones.month != "2026-10" || string(mustJSON(t, r.Data)["month"]) != `"2026-10"` {
		t.Errorf("report: %d %+v", code, r.Data)
	}
}
