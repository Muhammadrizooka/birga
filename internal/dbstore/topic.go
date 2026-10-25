package dbstore

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"

	"gitlab.com/loyihalar/birga/backend/internal/domain"
	"gitlab.com/loyihalar/birga/backend/internal/errs"
	"gitlab.com/loyihalar/birga/backend/pkg/logger"
)

// Every query filters on deleted_at IS NULL: a soft-deleted topic is invisible.
type topicRepo struct {
	store sqlClientProvider
}

type dbTopic struct {
	ID        string    `db:"id"`
	NameUz    string    `db:"name_uz"`
	NameRu    string    `db:"name_ru"`
	NameEn    string    `db:"name_en"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

const topicColumns = `id, name_uz, name_ru, name_en, created_at, updated_at`

func (r *topicRepo) Create(ctx context.Context, t domain.Topic) (domain.Topic, error) {
	l := logger.FromCtx(ctx, "topicRepo.Create")

	q := `INSERT INTO topics (name_uz, name_ru, name_en) VALUES ($1, $2, $3) RETURNING ` + topicColumns

	return r.one(ctx, l, q, t.NameUz, t.NameRu, t.NameEn)
}

func (r *topicRepo) Get(ctx context.Context, id string) (domain.Topic, error) {
	l := logger.FromCtx(ctx, "topicRepo.Get").With(zap.String("id", id))

	return r.one(ctx, l, `SELECT `+topicColumns+` FROM topics WHERE id = $1 AND deleted_at IS NULL`, id)
}

// List returns every active topic, oldest first. The list is short, so it is not paginated.
func (r *topicRepo) List(ctx context.Context) ([]domain.Topic, error) {
	l := logger.FromCtx(ctx, "topicRepo.List")

	q := `SELECT ` + topicColumns + ` FROM topics WHERE deleted_at IS NULL ORDER BY created_at, id`

	rows, err := r.store.sqlClientByCtx(ctx).Query(ctx, q)
	if err != nil {
		l.Error("sqlClient.Query", zap.Error(err))

		return nil, errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	dbRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbTopic])
	if err != nil {
		l.Error("pgx.CollectRows", zap.Error(err))

		return nil, errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	out := make([]domain.Topic, 0, len(dbRows))
	for i := range dbRows {
		out = append(out, dbRows[i].toDomain())
	}

	return out, nil
}

// CountActive returns how many of ids are active topics. ids must not repeat.
func (r *topicRepo) CountActive(ctx context.Context, ids []string) (int, error) {
	l := logger.FromCtx(ctx, "topicRepo.CountActive")

	var n int

	q := `SELECT COUNT(*) FROM topics WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL`
	if err := r.store.sqlClientByCtx(ctx).QueryRow(ctx, q, ids).Scan(&n); err != nil {
		l.Error("sqlClient.QueryRow", zap.Error(err))

		return 0, errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	return n, nil
}

// Update applies the non-nil fields of upd.
func (r *topicRepo) Update(ctx context.Context, id string, upd domain.TopicUpdate) (domain.Topic, error) {
	l := logger.FromCtx(ctx, "topicRepo.Update").With(zap.String("id", id))

	var (
		sets, args = topicSet(upd)
		q          = `UPDATE topics SET ` + sets +
			` WHERE id = $` + strconv.Itoa(len(args)+1) + ` AND deleted_at IS NULL RETURNING ` + topicColumns
	)

	return r.one(ctx, l, q, append(args, id)...)
}

// Delete soft-deletes the topic. Tips, articles and posts keep pointing at it (the row stays),
// so their tag still resolves for admins; the app lists only active topics.
func (r *topicRepo) Delete(ctx context.Context, id string) error {
	l := logger.FromCtx(ctx, "topicRepo.Delete").With(zap.String("id", id))

	tag, err := r.store.sqlClientByCtx(ctx).Exec(ctx,
		`UPDATE topics SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		l.Error("sqlClient.Exec", zap.Error(err))

		return errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	if tag.RowsAffected() == 0 {
		return errs.ErrTopicNotFound
	}

	return nil
}

// topicOrNull stores an optional topic id; nil and "" are NULL.
func topicOrNull(id *string) *string {
	if id == nil {
		return nil
	}

	return nullIfEmpty(*id)
}

// one runs q, which returns at most one topic row (SELECT, or a write with RETURNING). No row means
// ErrTopicNotFound; a unique-name violation means ErrTopicNameTaken.
func (r *topicRepo) one(ctx context.Context, l *zap.Logger, q string, args ...any) (domain.Topic, error) {
	rows, err := r.store.sqlClientByCtx(ctx).Query(ctx, q, args...)
	if err == nil {
		var g dbTopic

		g, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[dbTopic])
		if err == nil {
			return g.toDomain(), nil
		}
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Topic{}, errs.ErrTopicNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return domain.Topic{}, errs.Errf(errs.ErrTopicNameTaken, "%s: %s",
			errs.ErrTopicNameTaken.Error(), strings.TrimSuffix(strings.TrimPrefix(pgErr.ConstraintName, "topics_"), "_uniq"))
	}

	l.Error("topicRepo query failed", zap.Error(err), zap.String("query", q))

	return domain.Topic{}, errs.Errf(errs.ErrInternal, "%s", err.Error())
}

// topicSet builds a parameterized SET clause from fixed fragments; values never enter the SQL text.
func topicSet(upd domain.TopicUpdate) (string, []any) {
	var (
		sets = []string{setUpdatedAt}
		args []any
	)

	for _, f := range []struct {
		column string
		value  *string
	}{
		{"name_uz", upd.NameUz},
		{"name_ru", upd.NameRu},
		{"name_en", upd.NameEn},
	} {
		if f.value == nil {
			continue
		}

		args = append(args, *f.value)
		sets = append(sets, f.column+" = $"+strconv.Itoa(len(args)))
	}

	return strings.Join(sets, ", "), args
}

func (g dbTopic) toDomain() domain.Topic {
	return domain.Topic{
		ID:        g.ID,
		NameUz:    g.NameUz,
		NameRu:    g.NameRu,
		NameEn:    g.NameEn,
		CreatedAt: g.CreatedAt,
		UpdatedAt: g.UpdatedAt,
	}
}
