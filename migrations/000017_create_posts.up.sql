-- Video posts ("Kun videosi"). Texts come in Uzbek, Russian and English; suggestion_* is the
-- "Tavsiya" line and questions_* the "ask after watching" box, both optional ('' means none).
-- duration is in minutes. Only published posts reach the app; new ones start in_review.
CREATE TABLE IF NOT EXISTS posts (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title_uz      TEXT         NOT NULL,
    title_ru      TEXT         NOT NULL,
    title_en      TEXT         NOT NULL,
    video_id      UUID         NOT NULL REFERENCES media (id),
    goal_id       UUID         NOT NULL REFERENCES goals (id),
    min_age       SMALLINT     NOT NULL,
    max_age       SMALLINT     NOT NULL,
    duration      NUMERIC(6,2) NOT NULL,
    suggestion_uz TEXT         NOT NULL DEFAULT '',
    suggestion_ru TEXT         NOT NULL DEFAULT '',
    suggestion_en TEXT         NOT NULL DEFAULT '',
    questions_uz  TEXT         NOT NULL DEFAULT '',
    questions_ru  TEXT         NOT NULL DEFAULT '',
    questions_en  TEXT         NOT NULL DEFAULT '',
    status        TEXT         NOT NULL DEFAULT 'in_review',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ,

    CONSTRAINT posts_age_range_chk CHECK (min_age >= 2 AND max_age <= 6 AND min_age <= max_age),
    CONSTRAINT posts_duration_chk CHECK (duration > 0),
    CONSTRAINT posts_status_chk CHECK (status IN ('published', 'in_review', 'unpublished'))
);

CREATE INDEX IF NOT EXISTS posts_published_goal_idx ON posts (goal_id)
    WHERE status = 'published' AND deleted_at IS NULL;

-- The daily video of each user, one per local day (Uzbekistan time). A row is written the first
-- time the day is asked for, so the video stays the same all day; the rows are the user's video
-- history and decide which posts a free user may open.
CREATE TABLE IF NOT EXISTS user_daily_posts (
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    day        DATE        NOT NULL,
    post_id    UUID        NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (user_id, day)
);

CREATE INDEX IF NOT EXISTS user_daily_posts_post_idx ON user_daily_posts (user_id, post_id);
