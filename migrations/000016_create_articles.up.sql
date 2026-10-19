-- Short reading articles for parents. Texts come in Uzbek, Russian and English; tip_* is the
-- optional "try it today" box (empty means none). Each article serves one goal and an age range.
-- Only published articles reach the app; new ones start in_review.
CREATE TABLE IF NOT EXISTS articles (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title_uz           TEXT        NOT NULL,
    title_ru           TEXT        NOT NULL,
    title_en           TEXT        NOT NULL,
    article_body_uz    TEXT        NOT NULL,
    article_body_ru    TEXT        NOT NULL,
    article_body_en    TEXT        NOT NULL,
    tip_uz             TEXT        NOT NULL DEFAULT '',
    tip_ru             TEXT        NOT NULL DEFAULT '',
    tip_en             TEXT        NOT NULL DEFAULT '',
    photo_id           UUID REFERENCES media (id) ON DELETE SET NULL,
    expected_read_time SMALLINT    NOT NULL,
    min_age            SMALLINT    NOT NULL,
    max_age            SMALLINT    NOT NULL,
    goal_id            UUID        NOT NULL REFERENCES goals (id),
    status             TEXT        NOT NULL DEFAULT 'in_review',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at         TIMESTAMPTZ,

    CONSTRAINT articles_age_range_chk CHECK (min_age >= 2 AND max_age <= 6 AND min_age <= max_age),
    CONSTRAINT articles_read_time_chk CHECK (expected_read_time > 0),
    CONSTRAINT articles_status_chk CHECK (status IN ('published', 'in_review', 'unpublished'))
);

CREATE INDEX IF NOT EXISTS articles_published_goal_idx ON articles (goal_id)
    WHERE status = 'published' AND deleted_at IS NULL;

-- Articles a user marked as read ("O'qib chiqdim"). Deleting the user or the article row removes it.
CREATE TABLE IF NOT EXISTS article_reads (
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    article_id UUID        NOT NULL REFERENCES articles (id) ON DELETE CASCADE,
    read_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (user_id, article_id)
);

-- The daily article of each user, one per local day (Uzbekistan time). A row is written the
-- first time the day (or the day before it, for the "tomorrow" preview) is asked for, so the
-- article stays the same all day and the rows are the user's article history.
CREATE TABLE IF NOT EXISTS user_daily_articles (
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    day        DATE        NOT NULL,
    article_id UUID        NOT NULL REFERENCES articles (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (user_id, day)
);

-- Serves "when was this article last shown to the user" in the daily pick.
CREATE INDEX IF NOT EXISTS user_daily_articles_article_idx ON user_daily_articles (user_id, article_id);
