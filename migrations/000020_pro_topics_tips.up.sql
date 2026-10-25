-- Birga Pro by date: a user is Pro while pro_until is in the future (or their role is paid_user
-- or admin). An admin sets it by hand until payments exist.
ALTER TABLE users ADD COLUMN IF NOT EXISTS pro_until TIMESTAMPTZ;

-- Topics tag tips, articles and videos with subjects that are not goals ("Ovqat", "Uyqu",
-- "Ekran vaqti"). Names are unique per language among active topics, ignoring case.
CREATE TABLE IF NOT EXISTS topics (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name_uz    TEXT        NOT NULL,
    name_ru    TEXT        NOT NULL,
    name_en    TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS topics_name_uz_uniq ON topics (LOWER(name_uz)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS topics_name_ru_uniq ON topics (LOWER(name_ru)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS topics_name_en_uniq ON topics (LOWER(name_en)) WHERE deleted_at IS NULL;

-- The starter topics from the Figma library screens. Topics with these names that an admin
-- already made are kept.
INSERT INTO topics (name_uz, name_ru, name_en, created_at)
SELECT v.name_uz, v.name_ru, v.name_en, NOW() + v.n * INTERVAL '1 millisecond'
FROM (VALUES
    (1, 'Ovqat', 'Еда', 'Food'),
    (2, 'Uyqu', 'Сон', 'Sleep'),
    (3, 'Nutq', 'Речь', 'Speech'),
    (4, 'His-tuyg‘ular', 'Эмоции', 'Emotions'),
    (5, 'Ekran vaqti', 'Экранное время', 'Screen time'),
    (6, 'Xulq-atvor', 'Поведение', 'Behaviour'),
    (7, 'O‘yin', 'Игра', 'Play')
) AS v (n, name_uz, name_ru, name_en)
WHERE NOT EXISTS (
    SELECT 1 FROM topics t
    WHERE t.deleted_at IS NULL AND (LOWER(t.name_uz) = LOWER(v.name_uz) OR LOWER(t.name_ru) = LOWER(v.name_ru)
        OR LOWER(t.name_en) = LOWER(v.name_en))
);

-- Optional topic tag of articles and videos; the app shows it instead of the goal name when set.
ALTER TABLE articles ADD COLUMN IF NOT EXISTS topic_id UUID REFERENCES topics (id);
ALTER TABLE posts ADD COLUMN IF NOT EXISTS topic_id UUID REFERENCES topics (id);

-- Short tips for parents ("Kun maslahati", Kutubxona > Maslahat). Only published tips reach the app.
CREATE TABLE IF NOT EXISTS tips (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    text_uz    TEXT        NOT NULL,
    text_ru    TEXT        NOT NULL,
    text_en    TEXT        NOT NULL,
    topic_id   UUID        NOT NULL REFERENCES topics (id),
    min_age    SMALLINT    NOT NULL,
    max_age    SMALLINT    NOT NULL,
    status     TEXT        NOT NULL DEFAULT 'in_review',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT tips_age_range_chk CHECK (min_age >= 2 AND max_age <= 6 AND min_age <= max_age),
    CONSTRAINT tips_status_chk CHECK (status IN ('published', 'in_review', 'unpublished'))
);

CREATE INDEX IF NOT EXISTS tips_published_topic_idx ON tips (topic_id)
    WHERE status = 'published' AND deleted_at IS NULL;

-- The daily tip of each user, one per local day, like user_daily_articles.
CREATE TABLE IF NOT EXISTS user_daily_tips (
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    day        DATE        NOT NULL,
    tip_id     UUID        NOT NULL REFERENCES tips (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (user_id, day)
);

CREATE INDEX IF NOT EXISTS user_daily_tips_tip_idx ON user_daily_tips (user_id, tip_id);
