package dbstore

import (
	"context"
	"testing"
	"time"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
)

func TestDailyIDsAndMaxMinutes(t *testing.T) {
	s := newTestStore(t)

	inRollbackTx(t, s, func(ctx context.Context) {
		hideOtherArticles(t, s, ctx)

		goal := testGoal(t, s, ctx, "daily ids")
		mom, _ := parentAndChild(t, s, ctx, 3)
		repo := s.Article()

		short := article(goal, 2, 6, domain.ArticleStatusPublished)
		short.ReadMinutes = 2
		long := article(goal, 2, 6, domain.ArticleStatusPublished)
		long.ReadMinutes = 9

		a1, err := repo.Create(ctx, short)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		a2, err := repo.Create(ctx, long)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		today := domain.LocalDay(time.Now())
		if err := repo.SetDailyArticle(ctx, mom.ID, today, a1.ID); err != nil {
			t.Fatalf("SetDailyArticle: %v", err)
		}

		if err := repo.SetDailyArticle(ctx, mom.ID, today.AddDate(0, 0, 1), a2.ID); err != nil {
			t.Fatalf("SetDailyArticle: %v", err)
		}

		opened, err := repo.DailyIDs(ctx, mom.ID, []string{a1.ID, a2.ID}, today)
		if err != nil || !opened[a1.ID] || opened[a2.ID] {
			t.Fatalf("DailyIDs: %v %v (tomorrow's article must stay closed)", opened, err)
		}

		if ok, err := repo.WasDaily(ctx, mom.ID, a2.ID, today); err != nil || ok {
			t.Fatalf("WasDaily tomorrow: %v %v", ok, err)
		}

		items, total, err := repo.List(ctx, domain.ArticleFilter{MaxMinutes: 5, Limit: 10})
		if err != nil || total != 1 || items[0].ID != a1.ID {
			t.Fatalf("max_minutes: %d %+v %v", total, items, err)
		}
	})
}
