package dbstore

import (
	"context"
	"errors"
	"strconv"
	"time"

	pgx "github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
	"gitlab.com/loyihalar/birga/backend/pkg/logger"
)

// planRepo stores the 7-day plan: plan_themes (the same seven themes for everyone) and
// child_plan_days (the activity picked for each day of a child's current plan).
type planRepo struct {
	store sqlClientProvider
}

type dbPlanTheme struct {
	Day     int16   `db:"day"`
	TitleUz string  `db:"theme_title_uz"`
	TitleRu string  `db:"theme_title_ru"`
	Icon    string  `db:"theme_icon"`
	GoalID  *string `db:"theme_goal_id"`
}

type dbPlanRow struct {
	dbPlanTheme
	dbActivity

	StartedOn time.Time `db:"started_on"`
	Usable    bool      `db:"usable"`
}

const planThemeColumns = `t.day, t.title_uz AS theme_title_uz, t.title_ru AS theme_title_ru,
	t.icon AS theme_icon, t.goal_id AS theme_goal_id`

// Themes returns the plan themes in day order.
func (r *planRepo) Themes(ctx context.Context) ([]domain.PlanTheme, error) {
	l := logger.FromCtx(ctx, "planRepo.Themes")

	rows, err := r.store.sqlClientByCtx(ctx).Query(ctx, `SELECT `+planThemeColumns+` FROM plan_themes t ORDER BY t.day`)
	if err != nil {
		l.Error("sqlClient.Query", zap.Error(err))

		return nil, errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	dbRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbPlanTheme])
	if err != nil {
		l.Error("pgx.CollectRows", zap.Error(err))

		return nil, errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	out := make([]domain.PlanTheme, 0, len(dbRows))
	for i := range dbRows {
		out = append(out, dbRows[i].toDomain())
	}

	return out, nil
}

// LockChild locks the child row until the transaction ends, so two requests cannot build the
// same child's plan at once. A missing or deleted child is ErrChildNotFound.
func (r *planRepo) LockChild(ctx context.Context, childID string) error {
	l := logger.FromCtx(ctx, "planRepo.LockChild").With(zap.String("child_id", childID))

	var id string

	err := r.store.sqlClientByCtx(ctx).QueryRow(ctx,
		`SELECT id FROM children WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, childID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return errs.ErrChildNotFound
	}

	if err != nil {
		l.Error("sqlClient.QueryRow", zap.Error(err))

		return errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	return nil
}

// Items returns the child's current plan in day order, with the day the plan started. usable[i]
// is false when the activity of day i has since been unpublished or deleted. No plan means no items.
func (r *planRepo) Items(ctx context.Context, childID string) ([]domain.PlanItem, []bool, time.Time, error) {
	l := logger.FromCtx(ctx, "planRepo.Items").With(zap.String("child_id", childID))

	q := `
		SELECT ` + planThemeColumns + `, ` + prefixColumns("a", activityColumns) + `,
			p.started_on, (a.is_published AND a.deleted_at IS NULL) AS usable
		FROM child_plan_days p
		JOIN plan_themes t ON t.day = p.day
		JOIN activities a ON a.id = p.activity_id
		WHERE p.child_id = $1
		ORDER BY p.day`

	rows, err := r.store.sqlClientByCtx(ctx).Query(ctx, q, childID)
	if err != nil {
		l.Error("sqlClient.Query", zap.Error(err))

		return nil, nil, time.Time{}, errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	dbRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbPlanRow])
	if err != nil {
		l.Error("pgx.CollectRows", zap.Error(err))

		return nil, nil, time.Time{}, errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	var (
		items     = make([]domain.PlanItem, 0, len(dbRows))
		usable    = make([]bool, 0, len(dbRows))
		startedOn time.Time
	)

	for i := range dbRows {
		items = append(items, domain.PlanItem{Theme: dbRows[i].dbPlanTheme.toDomain(), Activity: dbRows[i].dbActivity.toDomain()})
		usable = append(usable, dbRows[i].Usable)
		startedOn = dbRows[i].StartedOn
	}

	return items, usable, startedOn, nil
}

// Pick returns one published activity for a plan day: within the child's age and not in
// ExcludeIDs. Activities serving the theme's goal come first, then those serving the parent's
// goals, then the ones the child has not done (or did longest ago); ties are broken per child and
// day. No match is ErrNoRecommendation.
func (r *planRepo) Pick(ctx context.Context, p domain.PlanPick) (domain.Activity, error) {
	l := logger.FromCtx(ctx, "planRepo.Pick").With(zap.String("child_id", p.ChildID), zap.Int("day", p.Day))

	q := `
		SELECT ` + prefixColumns("a", activityColumns) + `
		FROM activities a
		LEFT JOIN (
			SELECT activity_id, MAX(completed_on) AS last_on
			FROM activity_completions WHERE child_id = $1
			GROUP BY activity_id
		) done ON done.activity_id = a.id
		WHERE a.is_published AND a.deleted_at IS NULL AND a.min_age <= $2 AND a.max_age >= $2
			AND NOT (a.id = ANY($3::uuid[]))
		ORDER BY COALESCE($4::uuid = ANY(a.goal_ids), FALSE) DESC, a.goal_ids && $5::uuid[] DESC,
			done.last_on NULLS FIRST, md5(a.id::text || $1::text || $6::text)
		LIMIT 1`

	rows, err := r.store.sqlClientByCtx(ctx).Query(ctx, q,
		p.ChildID, p.Age, nonNil(p.ExcludeIDs), p.GoalID, nonNil(p.PreferGoalIDs), strconv.Itoa(p.Day))
	if err == nil {
		var a dbActivity

		a, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[dbActivity])
		if err == nil {
			return a.toDomain(), nil
		}
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Activity{}, errs.ErrNoRecommendation
	}

	l.Error("planRepo.Pick query failed", zap.Error(err))

	return domain.Activity{}, errs.Errf(errs.ErrInternal, "%s", err.Error())
}

// Replace makes items the child's plan, started on startedOn, dropping any earlier plan.
func (r *planRepo) Replace(ctx context.Context, childID string, startedOn time.Time, items []domain.PlanItem) error {
	l := logger.FromCtx(ctx, "planRepo.Replace").With(zap.String("child_id", childID))

	var (
		days        = make([]int16, 0, len(items))
		activityIDs = make([]string, 0, len(items))
		sqlClient   = r.store.sqlClientByCtx(ctx)
	)

	for _, it := range items {
		days = append(days, int16(it.Theme.Day)) //nolint:gosec // 1..7
		activityIDs = append(activityIDs, it.Activity.ID)
	}

	if _, err := sqlClient.Exec(ctx, `DELETE FROM child_plan_days WHERE child_id = $1`, childID); err != nil {
		l.Error("sqlClient.Exec delete", zap.Error(err))

		return errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	q := `
		INSERT INTO child_plan_days (child_id, day, activity_id, started_on)
		SELECT $1, d.day, d.activity_id, $4
		FROM unnest($2::smallint[], $3::uuid[]) AS d (day, activity_id)`

	if _, err := sqlClient.Exec(ctx, q, childID, days, activityIDs, startedOn.Format(time.DateOnly)); err != nil {
		l.Error("sqlClient.Exec insert", zap.Error(err))

		return errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	return nil
}

// SetActivity swaps the activity of one day of the child's plan.
func (r *planRepo) SetActivity(ctx context.Context, childID string, day int, activityID string) error {
	l := logger.FromCtx(ctx, "planRepo.SetActivity").With(zap.String("child_id", childID), zap.Int("day", day))

	_, err := r.store.sqlClientByCtx(ctx).Exec(ctx,
		`UPDATE child_plan_days SET activity_id = $3 WHERE child_id = $1 AND day = $2`, childID, day, activityID)
	if err != nil {
		l.Error("sqlClient.Exec", zap.Error(err))

		return errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	return nil
}

// CompletionDays returns, per activity, the days since `since` (inclusive) on which the child did it.
func (r *planRepo) CompletionDays(ctx context.Context, childID string, since time.Time) (map[string][]time.Time, error) {
	l := logger.FromCtx(ctx, "planRepo.CompletionDays").With(zap.String("child_id", childID))

	rows, err := r.store.sqlClientByCtx(ctx).Query(ctx,
		`SELECT activity_id, completed_on FROM activity_completions WHERE child_id = $1 AND completed_on >= $2`,
		childID, since.Format(time.DateOnly))
	if err != nil {
		l.Error("sqlClient.Query", zap.Error(err))

		return nil, errs.Errf(errs.ErrInternal, "%s", err.Error())
	}
	defer rows.Close()

	out := map[string][]time.Time{}

	for rows.Next() {
		var (
			id string
			on time.Time
		)

		if err := rows.Scan(&id, &on); err != nil {
			l.Error("rows.Scan", zap.Error(err))

			return nil, errs.Errf(errs.ErrInternal, "%s", err.Error())
		}

		out[id] = append(out[id], on)
	}

	if err := rows.Err(); err != nil {
		l.Error("rows.Err", zap.Error(err))

		return nil, errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	return out, nil
}

func (t dbPlanTheme) toDomain() domain.PlanTheme {
	return domain.PlanTheme{Day: int(t.Day), TitleUz: t.TitleUz, TitleRu: t.TitleRu, Icon: t.Icon, GoalID: t.GoalID}
}
