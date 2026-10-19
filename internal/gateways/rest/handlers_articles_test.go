package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

type fakeArticles struct {
	created    domain.Article
	updated    domain.ArticleUpdate
	filter     domain.ArticleFilter
	readerID   string
	readUser   string
	readID     string
	deletedID  string
	noTomorrow bool
	err        error
}

func (f *fakeArticles) useCases() ArticleUseCases {
	return ArticleUseCases{
		Creator: articleCreatorFunc(f.create), Lister: articleListerFunc(f.list), Getter: articleGetterFunc(f.get),
		Updater: articleUpdaterFunc(f.update), Deleter: articleDeleterFunc(f.delete), Reader: articleReaderFunc(f.read),
		Daily: f,
	}
}

func (f *fakeArticles) create(_ context.Context, a domain.Article) (domain.Article, error) {
	f.created = a
	a.ID = testID

	return a, f.err
}

func (f *fakeArticles) list(_ context.Context, fl domain.ArticleFilter) ([]domain.Article, int, error) {
	f.filter = fl

	return []domain.Article{{ID: testID, IsRead: fl.ReaderID != ""}}, 1, f.err
}

func (f *fakeArticles) get(_ context.Context, id, readerID string) (domain.Article, error) {
	f.readerID = readerID

	return domain.Article{ID: id}, f.err
}

func (f *fakeArticles) update(_ context.Context, id string, upd domain.ArticleUpdate) (domain.Article, error) {
	f.updated = upd

	return domain.Article{ID: id}, f.err
}

func (f *fakeArticles) delete(_ context.Context, id string) error {
	f.deletedID = id

	return f.err
}

func (f *fakeArticles) read(_ context.Context, userID, articleID string) error {
	f.readUser, f.readID = userID, articleID

	return f.err
}

func (f *fakeArticles) Execute(_ context.Context, userID string) (domain.DailyArticles, error) {
	f.readerID = userID
	d := domain.DailyArticles{Today: domain.DailyArticle{
		Day:     time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
		Article: domain.Article{ID: testID, TitleUz: "Bugun", IsRead: true},
	}}

	if !f.noTomorrow {
		d.Tomorrow = &domain.Article{ID: testUserID, TitleUz: "Ertaga", ReadMinutes: 4}
	}

	return d, f.err
}

func (f *fakeArticles) History(_ context.Context, userID string, limit, offset int) ([]domain.DailyArticle, int, error) {
	f.readerID = userID

	return []domain.DailyArticle{{Day: time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC), Article: domain.Article{ID: testID}}}, 1, f.err
}

type (
	articleCreatorFunc func(context.Context, domain.Article) (domain.Article, error)
	articleListerFunc  func(context.Context, domain.ArticleFilter) ([]domain.Article, int, error)
	articleGetterFunc  func(context.Context, string, string) (domain.Article, error)
	articleUpdaterFunc func(context.Context, string, domain.ArticleUpdate) (domain.Article, error)
	articleDeleterFunc func(context.Context, string) error
	articleReaderFunc  func(context.Context, string, string) error
)

func (f articleCreatorFunc) Execute(ctx context.Context, a domain.Article) (domain.Article, error) {
	return f(ctx, a)
}

func (f articleListerFunc) Execute(ctx context.Context, fl domain.ArticleFilter) ([]domain.Article, int, error) {
	return f(ctx, fl)
}

func (f articleGetterFunc) Execute(ctx context.Context, id, readerID string) (domain.Article, error) {
	return f(ctx, id, readerID)
}

func (f articleUpdaterFunc) Execute(ctx context.Context, id string, u domain.ArticleUpdate) (domain.Article, error) {
	return f(ctx, id, u)
}

func (f articleDeleterFunc) Execute(ctx context.Context, id string) error { return f(ctx, id) }

func (f articleReaderFunc) Execute(ctx context.Context, userID, id string) error {
	return f(ctx, userID, id)
}

func TestArticles_AdminCRUD(t *testing.T) {
	s, d := newTestServer(testAdminKey)

	body := `{"title_uz":"a","title_ru":"b","title_en":"c","article_body_uz":"d","article_body_ru":"e","article_body_en":"f",
		"tip_uz":"t","photo_id":"` + testID + `","expected_read_time":3,"min_age":2,"max_age":5,"goal_id":"` + testID + `","status":"published"}`
	if code, r := do(t, s, http.MethodPost, "/v1/admin/articles", body, adminJSON); code != http.StatusOK {
		t.Fatalf("create: %d %+v", code, r)
	}

	if a := d.arts.created; a.BodyEn != "f" || a.TipUz != "t" || a.ReadMinutes != 3 || a.GoalID != testID || a.Status != "published" {
		t.Errorf("create mapped wrong: %+v", a)
	}

	if code, _ := do(t, s, http.MethodGet, "/v1/admin/articles?status=in_review&q=yo&age=3", "", adminJSON); code != http.StatusOK ||
		d.arts.filter.Status != "in_review" || d.arts.filter.Search != "yo" || d.arts.filter.Age != 3 || d.arts.filter.ReaderID != "" {
		t.Errorf("admin list: %d %+v", code, d.arts.filter)
	}

	if code, _ := do(t, s, http.MethodGet, "/v1/admin/articles/"+testID, "", adminJSON); code != http.StatusOK || d.arts.readerID != "" {
		t.Errorf("admin get: %d reader %q", code, d.arts.readerID)
	}

	if code, _ := do(t, s, http.MethodPatch, "/v1/admin/articles/"+testID, `{"status":"unpublished","photo_id":""}`, adminJSON); code != http.StatusOK ||
		*d.arts.updated.Status != "unpublished" || *d.arts.updated.PhotoID != "" || d.arts.updated.TitleUz != nil {
		t.Errorf("update: %d %+v", code, d.arts.updated)
	}

	if code, _ := do(t, s, http.MethodDelete, "/v1/admin/articles/"+testID, "", adminJSON); code != http.StatusOK || d.arts.deletedID != testID {
		t.Errorf("delete: %d %q", code, d.arts.deletedID)
	}

	if code, _ := do(t, s, http.MethodGet, "/v1/admin/articles", "", nil); code != http.StatusUnauthorized {
		t.Errorf("admin without key: %d", code)
	}
}

func TestArticles_App(t *testing.T) {
	s, d := newTestServer(testAdminKey)
	auth := map[string]string{authorizationHeader: "Bearer good"}

	if code, _ := do(t, s, http.MethodGet, "/v1/articles?status=in_review", "", nil); code != http.StatusUnauthorized {
		t.Errorf("list without token: %d", code)
	}

	code, r := do(t, s, http.MethodGet, "/v1/articles?status=in_review", "", auth)
	if code != http.StatusOK || d.arts.filter.Status != domain.ArticleStatusPublished || d.arts.filter.ReaderID != testUserID {
		t.Fatalf("app list must be published only, for the reader: %d %+v", code, d.arts.filter)
	}

	var list userArticleListView
	mustDecode(t, r.Data, &list)

	if len(list.Items) != 1 || !list.Items[0].IsRead {
		t.Errorf("is_read not shown: %+v", list)
	}

	if code, _ := do(t, s, http.MethodGet, "/v1/articles/"+testID, "", auth); code != http.StatusOK || d.arts.readerID != testUserID {
		t.Errorf("get: %d reader %q", code, d.arts.readerID)
	}

	if code, _ := do(t, s, http.MethodPost, "/v1/articles/"+testID+"/read", "", auth); code != http.StatusOK ||
		d.arts.readUser != testUserID || d.arts.readID != testID {
		t.Errorf("read: %d %q %q", code, d.arts.readUser, d.arts.readID)
	}

	d.arts.err = errs.ErrArticleNotFound
	if code, _ := do(t, s, http.MethodPost, "/v1/articles/"+testID+"/read", "", auth); code != http.StatusNotFound {
		t.Errorf("read unknown: %d", code)
	}
}

func TestArticles_Daily(t *testing.T) {
	s, d := newTestServer(testAdminKey)
	auth := map[string]string{authorizationHeader: "Bearer good"}

	code, r := do(t, s, http.MethodGet, "/v1/me/articles/daily", "", auth)
	if code != http.StatusOK {
		t.Fatalf("daily: %d %+v", code, r)
	}

	var v struct {
		Day     string `json:"day"`
		Article struct {
			ID     string `json:"id"`
			IsRead bool   `json:"is_read"`
		} `json:"article"`
		Tomorrow *struct {
			TitleUz          string `json:"title_uz"`
			ExpectedReadTime int    `json:"expected_read_time"`
		} `json:"tomorrow"`
	}
	mustDecode(t, r.Data, &v)

	if v.Day != "2026-10-19" || v.Article.ID != testID || !v.Article.IsRead || v.Tomorrow == nil ||
		v.Tomorrow.TitleUz != "Ertaga" || v.Tomorrow.ExpectedReadTime != 4 {
		t.Errorf("daily view: %+v", v)
	}

	d.arts.noTomorrow = true
	if _, r := do(t, s, http.MethodGet, "/v1/me/articles/daily", "", auth); string(mustJSON(t, r.Data)["tomorrow"]) != "null" {
		t.Errorf("tomorrow must be null")
	}

	if code, r := do(t, s, http.MethodGet, "/v1/me/articles?limit=5", "", auth); code != http.StatusOK ||
		string(mustJSON(t, r.Data)["total"]) != "1" {
		t.Errorf("history: %d %+v", code, r)
	}

	d.arts.err = errs.ErrNoArticle
	if code, _ := do(t, s, http.MethodGet, "/v1/me/articles/daily", "", auth); code != http.StatusNotFound {
		t.Errorf("no article: %d", code)
	}
}

func mustDecode(t *testing.T, data any, into any) {
	t.Helper()

	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}

	if err := json.Unmarshal(b, into); err != nil {
		t.Fatal(err)
	}
}

func mustJSON(t *testing.T, data any) map[string]json.RawMessage {
	t.Helper()

	var m map[string]json.RawMessage
	mustDecode(t, data, &m)

	return m
}
