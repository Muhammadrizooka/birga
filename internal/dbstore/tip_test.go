package dbstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
)

func testTopic(t *testing.T, s *DBStore, ctx context.Context, name string) string {
	t.Helper()

	tp, err := s.Topic().Create(ctx, domain.Topic{NameUz: name + " uz", NameRu: name + " ru", NameEn: name + " en"})
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}

	return tp.ID
}

func TestTopicRepo_CRUD(t *testing.T) {
	s := newTestStore(t)

	inRollbackTx(t, s, func(ctx context.Context) {
		id := testTopic(t, s, ctx, "topic crud")

		upd, err := s.Topic().Update(ctx, id, domain.TopicUpdate{NameEn: strPtr("Renamed")})
		if err != nil || upd.NameEn != "Renamed" || upd.NameUz != "topic crud uz" {
			t.Fatalf("Update: %+v, %v", upd, err)
		}

		if err := s.Topic().Delete(ctx, id); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		if _, err := s.Topic().Get(ctx, id); !errors.Is(err, errs.ErrTopicNotFound) {
			t.Fatalf("deleted topic found: %v", err)
		}
	})

	inRollbackTx(t, s, func(ctx context.Context) {
		testTopic(t, s, ctx, "topic dup")

		if _, err := s.Topic().Create(ctx, domain.Topic{NameUz: "TOPIC DUP UZ", NameRu: "x", NameEn: "y"}); !errors.Is(err, errs.ErrTopicNameTaken) {
			t.Fatalf("duplicate name: %v", err)
		}
	})
}

func TestTipRepo_CRUDAndList(t *testing.T) {
	s := newTestStore(t)

	inRollbackTx(t, s, func(ctx context.Context) {
		if _, err := s.sqlClientByCtx(ctx).Exec(ctx, `DELETE FROM tips`); err != nil {
			t.Fatalf("cleanup: %v", err)
		}

		topic := testTopic(t, s, ctx, "tip crud")
		repo := s.Tip()

		tp, err := repo.Create(ctx, domain.Tip{TextUz: "Uxlashdan oldin", TextRu: "Перед сном", TextEn: "Before bed",
			TopicID: topic, MinAge: 2, MaxAge: 4, Status: domain.ArticleStatusInReview})
		if err != nil || tp.ID == "" || tp.TopicID != topic {
			t.Fatalf("Create: %+v, %v", tp, err)
		}

		upd, err := repo.Update(ctx, tp.ID, domain.TipUpdate{Status: strPtr(domain.ArticleStatusPublished), MaxAge: intPtr(6)})
		if err != nil || upd.Status != "published" || upd.MaxAge != 6 || upd.TextEn != "Before bed" {
			t.Fatalf("Update: %+v, %v", upd, err)
		}

		if _, err := repo.Update(ctx, tp.ID, domain.TipUpdate{MinAge: intPtr(6), MaxAge: intPtr(5)}); !errors.Is(err, errs.ErrValidation) {
			t.Fatalf("bad ages: %v", err)
		}
	})

	inRollbackTx(t, s, func(ctx context.Context) {
		if _, err := s.sqlClientByCtx(ctx).Exec(ctx, `DELETE FROM tips`); err != nil {
			t.Fatalf("cleanup: %v", err)
		}

		topic := testTopic(t, s, ctx, "tip list")
		mom, _ := parentAndChild(t, s, ctx, 3)
		repo := s.Tip()

		var ids []string

		for _, text := range []string{"Uyqu tartibi", "Ovqat vaqti"} {
			tp, err := repo.Create(ctx, domain.Tip{TextUz: text, TextRu: "r", TextEn: "e", TopicID: topic,
				MinAge: 2, MaxAge: 6, Status: domain.ArticleStatusPublished})
			if err != nil {
				t.Fatalf("Create: %v", err)
			}

			ids = append(ids, tp.ID)
		}

		if err := repo.SetDailyTip(ctx, mom.ID, domain.LocalDay(time.Now()), ids[0]); err != nil {
			t.Fatalf("SetDailyTip: %v", err)
		}

		items, total, err := repo.List(ctx, domain.TipFilter{Status: "published", LockFor: mom.ID, Limit: 10})
		if err != nil || total != 2 || len(items) != 2 {
			t.Fatalf("List: %d %+v %v", total, items, err)
		}

		for _, it := range items {
			if wantLocked := it.ID != ids[0]; it.Locked != wantLocked || (it.Locked && it.TextUz != "") || (!it.Locked && it.TextUz == "") {
				t.Errorf("tip %s: locked %v text %q", it.ID, it.Locked, it.TextUz)
			}
		}

		if items, total, err := repo.List(ctx, domain.TipFilter{Search: "ovqat", TopicID: topic, Age: 4, Limit: 10}); err != nil ||
			total != 1 || items[0].ID != ids[1] || items[0].Locked {
			t.Fatalf("search: %d %+v %v", total, items, err)
		}
	})
}

func TestTipRepo_Daily(t *testing.T) {
	s := newTestStore(t)

	inRollbackTx(t, s, func(ctx context.Context) {
		if _, err := s.sqlClientByCtx(ctx).Exec(ctx, `DELETE FROM tips`); err != nil {
			t.Fatalf("cleanup: %v", err)
		}

		topic := testTopic(t, s, ctx, "tip daily")
		mom, _ := parentAndChild(t, s, ctx, 3)
		repo := s.Tip()

		if _, ok, err := repo.PickDaily(ctx, mom.ID); err != nil || ok {
			t.Fatalf("pick with no tips: %v %v", ok, err)
		}

		create := func(minAge, maxAge, n int) string {
			tp, err := repo.Create(ctx, domain.Tip{TextUz: "t", TextRu: "t", TextEn: "t", TopicID: topic,
				MinAge: minAge, MaxAge: maxAge, Status: domain.ArticleStatusPublished})
			if err != nil {
				t.Fatalf("Create: %v", err)
			}

			if _, err := s.sqlClientByCtx(ctx).Exec(ctx,
				`UPDATE tips SET created_at = NOW() + make_interval(secs => $2) WHERE id = $1`, tp.ID, n); err != nil {
				t.Fatalf("set created_at: %v", err)
			}

			return tp.ID
		}

		tooOld := create(5, 6, 1)
		fits1 := create(2, 4, 2)
		fits2 := create(3, 3, 3)

		day := domain.LocalDay(time.Now())
		for i, want := range []string{fits1, fits2, fits1} {
			got, ok, err := repo.PickDaily(ctx, mom.ID)
			if err != nil || !ok || got != want {
				t.Fatalf("pick %d: got %s want %s (too old %s), %v", i, got, want, tooOld, err)
			}

			if err := repo.SetDailyTip(ctx, mom.ID, day.AddDate(0, 0, i), got); err != nil {
				t.Fatalf("SetDailyTip: %v", err)
			}
		}

		if id, ok, err := repo.DailyTipID(ctx, mom.ID, day); err != nil || !ok || id != fits1 {
			t.Fatalf("DailyTipID: %s %v %v", id, ok, err)
		}
	})
}

func TestUserRepo_ProUntil(t *testing.T) {
	s := newTestStore(t)

	inRollbackTx(t, s, func(ctx context.Context) {
		mom, _ := parentAndChild(t, s, ctx, 3)

		until := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)

		u, err := s.User().SetProUntil(ctx, mom.ID, &until)
		if err != nil || u.ProUntil == nil || !u.ProUntil.Equal(until) {
			t.Fatalf("SetProUntil: %+v, %v", u.ProUntil, err)
		}

		if u, err := s.User().SetProUntil(ctx, mom.ID, nil); err != nil || u.ProUntil != nil {
			t.Fatalf("clear: %+v, %v", u.ProUntil, err)
		}

		if _, err := s.User().SetProUntil(ctx, "00000000-0000-0000-0000-000000000000", nil); !errors.Is(err, errs.ErrUserNotFound) {
			t.Fatalf("unknown user: %v", err)
		}
	})
}
