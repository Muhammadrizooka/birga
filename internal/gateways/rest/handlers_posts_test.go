package rest

import (
	"context"
	"net/http"
	"testing"
	"time"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

type fakePosts struct {
	created domain.Post
	updated domain.PostUpdate
	filter  domain.PostFilter
	gotPro  *bool
	deleted string
	err     error
}

func (f *fakePosts) useCases() PostUseCases {
	return PostUseCases{Creator: f, Lister: postListerFunc(f.list), Getter: fakePostGetter{f}, Updater: postUpdaterFunc(f.update),
		Deleter: postDeleterFunc(f.delete), Daily: fakeDailyPost{f}}
}

func (f *fakePosts) Execute(_ context.Context, p domain.Post) (domain.Post, error) {
	f.created = p
	p.ID = testID

	return p, f.err
}

func (f *fakePosts) list(_ context.Context, fl domain.PostFilter) ([]domain.Post, int, error) {
	f.filter = fl

	return []domain.Post{{ID: testID, VideoID: testID, Duration: 3.5}}, 1, f.err
}

func (f *fakePosts) update(_ context.Context, id string, upd domain.PostUpdate) (domain.Post, error) {
	f.updated = upd

	return domain.Post{ID: id}, f.err
}

func (f *fakePosts) delete(_ context.Context, id string) error {
	f.deleted = id

	return f.err
}

type fakePostGetter struct{ f *fakePosts }

func (g fakePostGetter) Execute(_ context.Context, id string) (domain.Post, error) {
	return domain.Post{ID: id}, g.f.err
}

func (g fakePostGetter) ForUser(_ context.Context, _ string, pro bool, id string) (domain.Post, error) {
	g.f.gotPro = &pro

	return domain.Post{ID: id, VideoID: testUserID}, g.f.err
}

type fakeDailyPost struct{ f *fakePosts }

func (d fakeDailyPost) Execute(context.Context, string) (domain.DailyPosts, error) {
	return domain.DailyPosts{
		Today: domain.DailyPost{Day: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC), Post: domain.Post{ID: testID, VideoID: testUserID, Duration: 4.2}},
		Next:  []domain.Post{{ID: testUserID, TitleUz: "Hayvonlar ovozi", VideoID: testID}},
	}, d.f.err
}

func (d fakeDailyPost) History(context.Context, string, int, int) ([]domain.DailyPost, int, error) {
	return nil, 0, d.f.err
}

type (
	postListerFunc  func(context.Context, domain.PostFilter) ([]domain.Post, int, error)
	postUpdaterFunc func(context.Context, string, domain.PostUpdate) (domain.Post, error)
	postDeleterFunc func(context.Context, string) error
)

func (f postListerFunc) Execute(ctx context.Context, fl domain.PostFilter) ([]domain.Post, int, error) {
	return f(ctx, fl)
}

func (f postUpdaterFunc) Execute(ctx context.Context, id string, u domain.PostUpdate) (domain.Post, error) {
	return f(ctx, id, u)
}

func (f postDeleterFunc) Execute(ctx context.Context, id string) error { return f(ctx, id) }

func TestPosts_Admin(t *testing.T) {
	s, d := newTestServer(testAdminKey)

	body := `{"title_uz":"a","title_ru":"b","title_en":"c","video_id":"` + testID + `","goal_id":"` + testID + `",
		"min_age":3,"max_age":5,"duration":4.2,"questions_uz":"q1\nq2","status":"published"}`
	if code, r := do(t, s, http.MethodPost, "/v1/admin/posts", body, adminJSON); code != http.StatusOK {
		t.Fatalf("create: %d %+v", code, r)
	}

	if p := d.posts.created; p.Duration != 4.2 || p.QuestionsUz != "q1\nq2" || p.VideoID != testID || p.Status != "published" {
		t.Errorf("create mapped wrong: %+v", p)
	}

	if code, r := do(t, s, http.MethodGet, "/v1/admin/posts?status=in_review&age=4", "", adminJSON); code != http.StatusOK ||
		d.posts.filter.Status != "in_review" || d.posts.filter.Age != 4 || string(mustJSON(t, r.Data)["total"]) != "1" {
		t.Errorf("admin list: %d %+v", code, d.posts.filter)
	}

	if code, _ := do(t, s, http.MethodPatch, "/v1/admin/posts/"+testID, `{"duration":1.5,"suggestion_en":""}`, adminJSON); code != http.StatusOK ||
		*d.posts.updated.Duration != 1.5 || *d.posts.updated.SuggestionEn != "" || d.posts.updated.TitleUz != nil {
		t.Errorf("update: %d %+v", code, d.posts.updated)
	}

	if code, _ := do(t, s, http.MethodDelete, "/v1/admin/posts/"+testID, "", adminJSON); code != http.StatusOK || d.posts.deleted != testID {
		t.Errorf("delete: %d", code)
	}
}

func TestPosts_App(t *testing.T) {
	s, d := newTestServer(testAdminKey)

	if code, _ := do(t, s, http.MethodGet, "/v1/posts/"+testID, "", nil); code != http.StatusUnauthorized {
		t.Errorf("get without token: %d", code)
	}

	code, r := do(t, s, http.MethodGet, "/v1/posts/"+testID, "", bearer)
	if code != http.StatusOK || d.posts.gotPro == nil || *d.posts.gotPro ||
		string(mustJSON(t, r.Data)["video_url"]) != `"/v1/media/`+testUserID+`"` {
		t.Fatalf("get: %d pro %v %+v", code, d.posts.gotPro, r.Data)
	}

	if code, _ := do(t, s, http.MethodGet, "/v1/posts/"+testID, "", proBearer); code != http.StatusOK || !*d.posts.gotPro {
		t.Fatalf("get as pro: %d pro %v", code, *d.posts.gotPro)
	}

	code, r = do(t, s, http.MethodGet, "/v1/posts?status=in_review", "", bearer)
	if code != http.StatusOK || d.posts.filter.Status != domain.ArticleStatusPublished {
		t.Fatalf("list must be published only: %d %+v", code, d.posts.filter)
	}

	var list struct {
		Items []map[string]any `json:"items"`
	}
	mustDecode(t, r.Data, &list)

	if _, hasVideo := list.Items[0]["video_url"]; hasVideo || list.Items[0]["duration"] != 3.5 {
		t.Errorf("list items must be previews without the video: %+v", list.Items[0])
	}

	code, r = do(t, s, http.MethodGet, "/v1/me/posts/daily", "", bearer)

	var daily struct {
		Day  string `json:"day"`
		Post struct {
			VideoURL string  `json:"video_url"`
			Duration float64 `json:"duration"`
		} `json:"post"`
		Next []map[string]any `json:"next"`
	}
	mustDecode(t, r.Data, &daily)

	if code != http.StatusOK || daily.Day != "2026-10-19" || daily.Post.Duration != 4.2 || daily.Post.VideoURL == "" ||
		len(daily.Next) != 1 || daily.Next[0]["title_uz"] != "Hayvonlar ovozi" || daily.Next[0]["video_url"] != nil {
		t.Errorf("daily: %d %+v", code, daily)
	}

	if code, r := do(t, s, http.MethodGet, "/v1/me/posts", "", bearer); code != http.StatusOK || string(mustJSON(t, r.Data)["items"]) != "[]" {
		t.Errorf("history: %d %+v", code, r)
	}

	d.posts.err = errs.ErrPostNotOpened
	if code, _ := do(t, s, http.MethodGet, "/v1/posts/"+testID, "", bearer); code != http.StatusForbidden {
		t.Errorf("not opened: %d", code)
	}

	d.posts.err = errs.ErrNoPost
	if code, _ := do(t, s, http.MethodGet, "/v1/me/posts/daily", "", bearer); code != http.StatusNotFound {
		t.Errorf("no video: %d", code)
	}
}
