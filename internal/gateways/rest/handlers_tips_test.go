package rest

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
)

type fakeTips struct {
	createdTopic domain.Topic
	created      domain.Tip
	updated      domain.TipUpdate
	filter       domain.TipFilter
	appOnly      bool
	principal    domain.Principal
	grantUntil   *time.Time
	err          error
}

func (f *fakeTips) useCases() TipUseCases {
	return TipUseCases{Topics: fakeTopicManager{f}, Tips: fakeTipManager{f}, Daily: f}
}

type fakeTopicManager struct{ f *fakeTips }

func (m fakeTopicManager) Create(_ context.Context, t domain.Topic) (domain.Topic, error) {
	m.f.createdTopic = t
	t.ID = testID

	return t, m.f.err
}

func (m fakeTopicManager) List(context.Context) ([]domain.Topic, error) {
	return []domain.Topic{{ID: testID, NameEn: "Sleep"}}, m.f.err
}

func (m fakeTopicManager) Update(_ context.Context, id string, _ domain.TopicUpdate) (domain.Topic, error) {
	return domain.Topic{ID: id}, m.f.err
}

func (m fakeTopicManager) Delete(context.Context, string) error { return m.f.err }

type fakeTipManager struct{ f *fakeTips }

func (m fakeTipManager) Create(_ context.Context, t domain.Tip) (domain.Tip, error) {
	m.f.created = t
	t.ID = testID

	return t, m.f.err
}

func (m fakeTipManager) Get(_ context.Context, id string, appOnly bool) (domain.Tip, error) {
	m.f.appOnly = appOnly

	return domain.Tip{ID: id}, m.f.err
}

func (m fakeTipManager) Update(_ context.Context, id string, upd domain.TipUpdate) (domain.Tip, error) {
	m.f.updated = upd

	return domain.Tip{ID: id}, m.f.err
}

func (m fakeTipManager) Delete(context.Context, string) error { return m.f.err }

func (m fakeTipManager) List(_ context.Context, fl domain.TipFilter) ([]domain.Tip, int, error) {
	m.f.filter = fl

	return []domain.Tip{{ID: testID, Locked: fl.LockFor != ""}}, 1, m.f.err
}

func (f *fakeTips) Execute(context.Context, string) (domain.DailyTip, error) {
	return domain.DailyTip{Day: time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC), Tip: domain.Tip{ID: testID, TextUz: "maslahat"}}, f.err
}

func (f *fakeTips) Offer(_ context.Context, p domain.Principal) (domain.ProOffer, error) {
	f.principal = p

	return domain.ProOffer{PriceUZS: 16000, Plan: domain.PlanFree}, f.err
}

func (f *fakeTips) Grant(_ context.Context, userID string, until *time.Time) (domain.User, error) {
	f.grantUntil = until

	return domain.User{ID: userID, ProUntil: until}, f.err
}

func (f *fakeTips) Purchase(_ context.Context, p domain.Principal, method string) (domain.ProPurchase, error) {
	f.principal = p
	until := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)

	return domain.ProPurchase{PaymentMethod: method, AmountUZS: 16000, User: domain.User{ID: p.UserID, ProUntil: &until}}, f.err
}

func TestProPurchase(t *testing.T) {
	s, d := newTestServer(testAdminKey)

	code, r := do(t, s, http.MethodPost, "/v1/pro/purchase", `{"payment_method":"click"}`, bearer)
	data := mustJSON(t, r.Data)
	if code != http.StatusOK || d.tips.principal.UserID != testUserID || string(data["plan"]) != `"pro"` ||
		string(data["paid"]) != "false" || string(data["payment_method"]) != `"click"` {
		t.Fatalf("purchase: %d %+v", code, r.Data)
	}

	if code, _ := do(t, s, http.MethodPost, "/v1/pro/purchase", `{"payment_method":"click"}`, nil); code != http.StatusUnauthorized {
		t.Errorf("without token: %d", code)
	}
}

func TestTips_Admin(t *testing.T) {
	s, d := newTestServer(testAdminKey)

	body := `{"text_uz":"a","text_ru":"b","text_en":"c","topic_id":"` + testID + `","min_age":2,"max_age":6,"status":"published"}`
	if code, r := do(t, s, http.MethodPost, "/v1/admin/tips", body, adminJSON); code != http.StatusOK {
		t.Fatalf("create: %d %+v", code, r)
	}

	if tp := d.tips.created; tp.TextEn != "c" || tp.TopicID != testID || tp.MaxAge != 6 || tp.Status != "published" {
		t.Errorf("create mapped wrong: %+v", tp)
	}

	if code, _ := do(t, s, http.MethodGet, "/v1/admin/tips?status=draft&topic_id="+testID+"&age=3", "", adminJSON); code != http.StatusOK ||
		d.tips.filter.Status != "draft" || d.tips.filter.TopicID != testID || d.tips.filter.Age != 3 || d.tips.filter.LockFor != "" {
		t.Errorf("admin list: %d %+v", code, d.tips.filter)
	}

	if code, _ := do(t, s, http.MethodGet, "/v1/admin/tips/"+testID, "", adminJSON); code != http.StatusOK || d.tips.appOnly {
		t.Errorf("admin get: %d appOnly %v", code, d.tips.appOnly)
	}

	if code, _ := do(t, s, http.MethodPatch, "/v1/admin/tips/"+testID, `{"status":"unpublished"}`, adminJSON); code != http.StatusOK ||
		*d.tips.updated.Status != "unpublished" || d.tips.updated.TextUz != nil {
		t.Errorf("update: %d %+v", code, d.tips.updated)
	}

	if code, _ := do(t, s, http.MethodPost, "/v1/admin/topics", `{"name_uz":"Uyqu","name_ru":"Сон","name_en":"Sleep"}`, adminJSON); code != http.StatusOK ||
		d.tips.createdTopic.NameRu != "Сон" {
		t.Errorf("create topic: %d %+v", code, d.tips.createdTopic)
	}

	if code, _ := do(t, s, http.MethodGet, "/v1/topics", "", nil); code != http.StatusOK {
		t.Errorf("public topics: %d", code)
	}

	if code, _ := do(t, s, http.MethodPost, "/v1/admin/tips", body, nil); code != http.StatusUnauthorized {
		t.Errorf("admin without key: %d", code)
	}
}

func TestTips_App(t *testing.T) {
	s, d := newTestServer(testAdminKey)

	code, r := do(t, s, http.MethodGet, "/v1/tips?status=in_review", "", bearer)
	if code != http.StatusOK || d.tips.filter.Status != domain.ArticleStatusPublished || d.tips.filter.LockFor != testUserID ||
		!strings.Contains(string(mustJSON(t, r.Data)["items"]), `"locked":true`) {
		t.Fatalf("free list: %d %+v %+v", code, d.tips.filter, r.Data)
	}

	if code, _ := do(t, s, http.MethodGet, "/v1/tips", "", proBearer); code != http.StatusOK || d.tips.filter.LockFor != "" {
		t.Errorf("pro list: %d %+v", code, d.tips.filter)
	}

	code, r = do(t, s, http.MethodGet, "/v1/me/tips/daily", "", bearer)
	if code != http.StatusOK || string(mustJSON(t, r.Data)["day"]) != `"2026-10-25"` {
		t.Errorf("daily: %d %+v", code, r.Data)
	}

	if code, _ := do(t, s, http.MethodGet, "/v1/me/tips/daily", "", nil); code != http.StatusUnauthorized {
		t.Errorf("daily without token: %d", code)
	}
}

func TestPro(t *testing.T) {
	s, d := newTestServer(testAdminKey)

	code, r := do(t, s, http.MethodGet, "/v1/pro", "", proBearer)
	if code != http.StatusOK || !d.tips.principal.Pro || d.tips.principal.UserID != testUserID ||
		string(mustJSON(t, r.Data)["price_uzs"]) != "16000" {
		t.Fatalf("offer: %d %+v %+v", code, d.tips.principal, r.Data)
	}

	code, r = do(t, s, http.MethodPut, "/v1/admin/users/"+testID+"/pro", `{"until":"2099-01-01T00:00:00+05:00"}`, adminJSON)
	if code != http.StatusOK || d.tips.grantUntil == nil || string(mustJSON(t, r.Data)["plan"]) != `"pro"` {
		t.Fatalf("grant: %d %v %+v", code, d.tips.grantUntil, r.Data)
	}

	if code, _ := do(t, s, http.MethodPut, "/v1/admin/users/"+testID+"/pro", `{"until":null}`, adminJSON); code != http.StatusOK ||
		d.tips.grantUntil != nil {
		t.Errorf("end: %d %v", code, d.tips.grantUntil)
	}
}
