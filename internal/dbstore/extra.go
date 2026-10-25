package dbstore

import (
	"context"
	"errors"
	"time"

	pgx "github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
	"gitlab.com/loyihalar/birga/backend/pkg/logger"
)

// extraRepo stores the activities parents add to a child's day (child_day_extras).
type extraRepo struct {
	store sqlClientProvider
}

type dbDayExtra struct {
	dbActivity
	Done    bool      `db:"done"`
	AddedAt time.Time `db:"added_at"`
}

// ListDay returns the activities added to the child's day, oldest first, each with whether the
// child completed it that day. Deleted activities are left out.
func (r *extraRepo) ListDay(ctx context.Context, childID string, day time.Time) ([]domain.DayExtra, error) {
	l := logger.FromCtx(ctx, "extraRepo.ListDay").With(zap.String("child_id", childID))

	q := `
		SELECT ` + prefixColumns("a", activityColumns) + `, e.added_at,
			EXISTS (SELECT 1 FROM activity_completions c
				WHERE c.child_id = e.child_id AND c.activity_id = e.activity_id AND c.completed_on = e.day) AS done
		FROM child_day_extras e
		JOIN activities a ON a.id = e.activity_id AND a.deleted_at IS NULL
		WHERE e.child_id = $1 AND e.day = $2
		ORDER BY e.added_at, a.id`

	rows, err := r.store.sqlClientByCtx(ctx).Query(ctx, q, childID, day.Format(time.DateOnly))
	if err != nil {
		l.Error("sqlClient.Query", zap.Error(err))

		return nil, errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	dbRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbDayExtra])
	if err != nil {
		l.Error("pgx.CollectRows", zap.Error(err))

		return nil, errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	out := make([]domain.DayExtra, 0, len(dbRows))
	for i := range dbRows {
		out = append(out, domain.DayExtra{Activity: dbRows[i].toDomain(), Done: dbRows[i].Done, AddedAt: dbRows[i].AddedAt})
	}

	return out, nil
}

// Add puts the activity on the child's day; adding it again changes nothing.
func (r *extraRepo) Add(ctx context.Context, childID string, day time.Time, activityID string) error {
	l := logger.FromCtx(ctx, "extraRepo.Add").With(zap.String("child_id", childID))

	_, err := r.store.sqlClientByCtx(ctx).Exec(ctx, `
		INSERT INTO child_day_extras (child_id, day, activity_id) VALUES ($1, $2, $3)
		ON CONFLICT (child_id, day, activity_id) DO NOTHING`, childID, day.Format(time.DateOnly), activityID)
	if err != nil {
		l.Error("sqlClient.Exec", zap.Error(err))

		return errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	return nil
}

// Replace swaps oldID for newID in the child's day, keeping its place in the list.
func (r *extraRepo) Replace(ctx context.Context, childID string, day time.Time, oldID, newID string) error {
	l := logger.FromCtx(ctx, "extraRepo.Replace").With(zap.String("child_id", childID))

	_, err := r.store.sqlClientByCtx(ctx).Exec(ctx, `
		UPDATE child_day_extras SET activity_id = $4
		WHERE child_id = $1 AND day = $2 AND activity_id = $3`, childID, day.Format(time.DateOnly), oldID, newID)
	if err != nil {
		l.Error("sqlClient.Exec", zap.Error(err))

		return errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	return nil
}

// Pick returns a published activity serving one of p.GoalIDs (any goal when empty) within p.Age
// and not in p.ExcludeIDs. With p.Difficulty only activities of that level match; otherwise normal
// ones come first. Then those sharing more of p.GoalIDs, the ones the child never did, the one
// done longest ago, and a per-child, per-day order. No match is errs.ErrNoExtra.
func (r *extraRepo) Pick(ctx context.Context, p domain.ExtraPick, day time.Time) (domain.Activity, error) {
	l := logger.FromCtx(ctx, "extraRepo.Pick").With(zap.String("child_id", p.ChildID))

	q := `
		SELECT ` + prefixColumns("a", activityColumns) + `
		FROM activities a
		LEFT JOIN (
			SELECT activity_id, MAX(completed_on) AS last_on
			FROM activity_completions WHERE child_id = $1
			GROUP BY activity_id
		) done ON done.activity_id = a.id
		WHERE a.is_published AND a.deleted_at IS NULL AND a.min_age <= $2 AND a.max_age >= $2
			AND (cardinality($3::uuid[]) = 0 OR a.goal_ids && $3::uuid[]) AND NOT (a.id = ANY($4::uuid[]))
			AND ($6 = '' OR a.difficulty = $6)
		ORDER BY a.difficulty = 'normal' DESC,
			(SELECT COUNT(*) FROM unnest(a.goal_ids) g WHERE g = ANY($3::uuid[])) DESC,
			done.last_on NULLS FIRST, md5(a.id::text || $1::text || $5::text)
		LIMIT 1`

	rows, err := r.store.sqlClientByCtx(ctx).Query(ctx, q,
		p.ChildID, p.Age, nonNil(p.GoalIDs), nonNil(p.ExcludeIDs), day.Format(time.DateOnly), p.Difficulty)
	if err == nil {
		var a dbActivity

		a, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[dbActivity])
		if err == nil {
			return a.toDomain(), nil
		}
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Activity{}, errs.ErrNoExtra
	}

	l.Error("extraRepo.Pick query failed", zap.Error(err))

	return domain.Activity{}, errs.Errf(errs.ErrInternal, "%s", err.Error())
}
