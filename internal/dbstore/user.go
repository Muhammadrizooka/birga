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

const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

// Every query filters on deleted_at IS NULL: a soft-deleted user is invisible.
type userRepo struct {
	store sqlClientProvider
}

type dbUser struct {
	ID                 string     `db:"id"`
	Name               *string    `db:"name"`
	Username           *string    `db:"username"`
	PhoneNumber        *string    `db:"phone_number"`
	PhotoID            *string    `db:"photo_id"`
	Relationship       *string    `db:"relationship"`
	AgreedOnConditions bool       `db:"agreed_on_conditions"`
	ReminderEnabled    bool       `db:"reminder_enabled"`
	ReminderTime       string     `db:"reminder_time"`
	ProUntil           *time.Time `db:"pro_until"`
	Role               *string    `db:"role"`
	CreatedAt          time.Time  `db:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at"`
}

const userColumns = `id, name, username, phone_number, photo_id, relationship, ` +
	`agreed_on_conditions, reminder_enabled, reminder_time, pro_until, created_at, updated_at`

// userSelect is userColumns plus the user's role from user_auth, for queries on the users table.
var userSelect = userColumns + ", " + userRoleColumn("users")

// userRoleColumn selects the role of the user row aliased alias; NULL without a user_auth row.
func userRoleColumn(alias string) string {
	return `(SELECT ua.role::text FROM user_auth ua WHERE ua.id = ` + alias + `.id) AS role`
}

func (r *userRepo) Create(ctx context.Context, u domain.User) (domain.User, error) {
	l := logger.FromCtx(ctx, "userRepo.Create")

	q := `
		INSERT INTO users (name, username, phone_number, photo_id, relationship, agreed_on_conditions)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING ` + userSelect

	return r.one(ctx, l, q, u.Name, u.Username, u.PhoneNumber, u.PhotoID, u.Relationship, u.AgreedOnConditions)
}

func (r *userRepo) Get(ctx context.Context, id string) (domain.User, error) {
	l := logger.FromCtx(ctx, "userRepo.Get").With(zap.String("id", id))

	return r.one(ctx, l, `SELECT `+userSelect+` FROM users WHERE id = $1 AND deleted_at IS NULL`, id)
}

// List returns one page of active users and the total number of active users.
func (r *userRepo) List(ctx context.Context, f domain.UserFilter) ([]domain.User, int, error) {
	l := logger.FromCtx(ctx, "userRepo.List")

	var (
		sqlClient = r.store.sqlClientByCtx(ctx)
		where     = `deleted_at IS NULL AND ($1 = '' OR username ILIKE $1 || '%' OR name ILIKE '%' || $1 || '%'
			OR phone_number LIKE '%' || $1 || '%')`
		countQ = `SELECT COUNT(*) FROM users WHERE ` + where
		listQ  = `SELECT ` + userSelect + ` FROM users WHERE ` + where + `
			ORDER BY username IS NOT DISTINCT FROM lower($2) DESC, created_at DESC, id LIMIT $3 OFFSET $4`
		q = likeEscaper.Replace(f.Query)
	)

	var total int
	if err := sqlClient.QueryRow(ctx, countQ, q).Scan(&total); err != nil {
		l.Error("sqlClient.QueryRow", zap.Error(err))

		return nil, 0, errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	rows, err := sqlClient.Query(ctx, listQ, q, f.Query, f.Limit, f.Offset)
	if err != nil {
		l.Error("sqlClient.Query", zap.Error(err))

		return nil, 0, errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	dbRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbUser])
	if err != nil {
		l.Error("pgx.CollectRows", zap.Error(err))

		return nil, 0, errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	out := make([]domain.User, 0, len(dbRows))
	for i := range dbRows {
		out = append(out, dbRows[i].toDomain())
	}

	return out, total, nil
}

// Update applies the non-nil fields of upd. A field pointing to "" is stored as NULL.
func (r *userRepo) Update(ctx context.Context, id string, upd domain.UserUpdate) (domain.User, error) {
	l := logger.FromCtx(ctx, "userRepo.Update").With(zap.String("id", id))

	var (
		sets, args = userSet(upd)
		q          = `UPDATE users SET ` + sets +
			` WHERE id = $` + strconv.Itoa(len(args)+1) + ` AND deleted_at IS NULL RETURNING ` + userSelect
	)

	return r.one(ctx, l, q, append(args, id)...)
}

// SetProUntil sets or clears (nil) the date Birga Pro given by hand ends. When until is not after
// now, Pro ends: a paid_user role becomes user too, so nothing keeps the user Pro (admins stay).
func (r *userRepo) SetProUntil(ctx context.Context, id string, until *time.Time) (domain.User, error) {
	l := logger.FromCtx(ctx, "userRepo.SetProUntil").With(zap.String("id", id))

	q := `
		WITH role_end AS (
			UPDATE user_auth SET role = 'user'
			WHERE id = $2 AND role = 'paid_user' AND ($1::timestamptz IS NULL OR $1::timestamptz <= now())
			RETURNING id
		)
		UPDATE users SET pro_until = $1, ` + setUpdatedAt + `
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING ` + userColumns + `,
			CASE WHEN EXISTS (SELECT 1 FROM role_end) THEN 'user' ELSE ` +
		`(SELECT ua.role::text FROM user_auth ua WHERE ua.id = users.id) END AS role`

	return r.one(ctx, l, q, until, id)
}

// one runs q, which returns at most one user row (SELECT, or a write with RETURNING). No row means ErrUserNotFound;
// a unique-index violation means the matching conflict error.
func (r *userRepo) one(ctx context.Context, l *zap.Logger, q string, args ...any) (domain.User, error) {
	rows, err := r.store.sqlClientByCtx(ctx).Query(ctx, q, args...)
	if err == nil {
		var u dbUser

		u, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[dbUser])
		if err == nil {
			return u.toDomain(), nil
		}
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, errs.ErrUserNotFound
	}

	if conflict := userConflict(err); conflict != nil {
		return domain.User{}, conflict
	}

	l.Error("userRepo query failed", zap.Error(err), zap.String("query", q))

	return domain.User{}, errs.Errf(errs.ErrInternal, "%s", err.Error())
}

// Delete soft-deletes the user. The users_soft_delete_auth_trg trigger removes the user_auth row.
func (r *userRepo) Delete(ctx context.Context, id string) error {
	l := logger.FromCtx(ctx, "userRepo.Delete").With(zap.String("id", id))

	var (
		sqlClient = r.store.sqlClientByCtx(ctx)
		q         = `UPDATE users SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	)

	tag, err := sqlClient.Exec(ctx, q, id)
	if err != nil {
		l.Error("sqlClient.Exec", zap.Error(err))

		return errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	if tag.RowsAffected() == 0 {
		return errs.ErrUserNotFound
	}

	return nil
}

// userSet builds a parameterized SET clause from fixed fragments; values never enter the SQL text.
func userSet(upd domain.UserUpdate) (string, []any) {
	var (
		sets = []string{setUpdatedAt}
		args []any
	)

	for _, f := range []struct {
		column string
		value  *string
	}{
		{"name", upd.Name},
		{"username", upd.Username},
		{"phone_number", upd.PhoneNumber},
		{"photo_id", upd.PhotoID},
	} {
		if f.value == nil {
			continue
		}

		args = append(args, nullIfEmpty(*f.value))
		sets = append(sets, f.column+" = $"+strconv.Itoa(len(args)))
	}

	for _, f := range []struct {
		column string
		value  any
		set    bool
	}{
		{"agreed_on_conditions", upd.AgreedOnConditions, upd.AgreedOnConditions != nil},
		{"reminder_enabled", upd.ReminderEnabled, upd.ReminderEnabled != nil},
		{"reminder_time", upd.ReminderTime, upd.ReminderTime != nil},
	} {
		if !f.set {
			continue
		}

		args = append(args, f.value)
		sets = append(sets, f.column+" = $"+strconv.Itoa(len(args)))
	}

	return strings.Join(sets, ", "), args
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

// userConflict maps a unique-index violation (users or user_auth) to the matching conflict error, and a photo_id
// with no media row to ErrPhotoNotFound. It returns nil for any other error.
func userConflict(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return nil
	}

	if pgErr.Code == pgForeignKeyViolation && pgErr.ConstraintName == "users_photo_id_fkey" {
		return errs.ErrPhotoNotFound
	}

	if pgErr.Code != pgUniqueViolation {
		return nil
	}

	switch pgErr.ConstraintName {
	case "users_username_uniq", "user_auth_username_uniq":
		return errs.ErrUsernameTaken
	case "users_phone_number_uniq":
		return errs.ErrPhoneNumberTaken
	default:
		return errs.Errf(errs.ErrConflict, "%s", pgErr.Message)
	}
}

func (u dbUser) toDomain() domain.User {
	return domain.User{
		ID:           u.ID,
		Name:         u.Name,
		Username:     u.Username,
		PhoneNumber:  u.PhoneNumber,
		PhotoID:      u.PhotoID,
		Relationship: u.Relationship,

		AgreedOnConditions: u.AgreedOnConditions,
		ReminderEnabled:    u.ReminderEnabled,
		ReminderTime:       u.ReminderTime,
		ProUntil:           u.ProUntil,
		Role:               domain.UserRole(deref(u.Role)),
		CreatedAt:          u.CreatedAt,
		UpdatedAt:          u.UpdatedAt,
	}
}

// ExistsByPhone reports whether an active user has this phone number.
func (r *userRepo) ExistsByPhone(ctx context.Context, phone string) (bool, error) {
	l := logger.FromCtx(ctx, "userRepo.ExistsByPhone")

	var exists bool

	q := `SELECT EXISTS (SELECT 1 FROM users WHERE phone_number = $1 AND deleted_at IS NULL)`
	if err := r.store.sqlClientByCtx(ctx).QueryRow(ctx, q, phone).Scan(&exists); err != nil {
		l.Error("sqlClient.QueryRow", zap.Error(err))

		return false, errs.Errf(errs.ErrInternal, "%s", err.Error())
	}

	return exists, nil
}

// GetByPhone returns the active user with this phone number, or ErrUserNotFound.
func (r *userRepo) GetByPhone(ctx context.Context, phone string) (domain.User, error) {
	l := logger.FromCtx(ctx, "userRepo.GetByPhone")

	return r.one(ctx, l, `SELECT `+userSelect+` FROM users WHERE phone_number = $1 AND deleted_at IS NULL`, phone)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
