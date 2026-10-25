package postgetter

import (
	"context"
	"errors"
	"testing"
	"time"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

type fakeRepo struct {
	post     domain.Post
	wasDaily bool
}

func (f fakeRepo) Get(context.Context, string) (domain.Post, error) { return f.post, nil }

func (f fakeRepo) WasDaily(context.Context, string, string, time.Time) (bool, error) {
	return f.wasDaily, nil
}

func TestForUser(t *testing.T) {
	published := domain.Post{ID: "p", Status: domain.ArticleStatusPublished}

	cases := []struct {
		name     string
		post     domain.Post
		pro      bool
		wasDaily bool
		want     error
	}{
		{"free user, own daily video", published, false, true, nil},
		{"free user, other video", published, false, false, errs.ErrForbidden},
		{"pro user, any video", published, true, false, nil},
		{"pro user, unpublished", domain.Post{Status: domain.ArticleStatusInReview}, true, true, errs.ErrNotFound},
	}

	for _, tc := range cases {
		_, err := New(nil, fakeRepo{post: tc.post, wasDaily: tc.wasDaily}).ForUser(context.Background(), "u", tc.pro, "p")
		if (tc.want == nil) != (err == nil) || (tc.want != nil && !errors.Is(err, tc.want)) {
			t.Errorf("%s: got %v, want %v", tc.name, err, tc.want)
		}
	}
}
