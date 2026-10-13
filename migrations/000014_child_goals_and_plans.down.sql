DROP TABLE IF EXISTS child_plan_days;
DROP TABLE IF EXISTS child_plans;

ALTER TABLE users ADD COLUMN IF NOT EXISTS goal_ids UUID[] NOT NULL DEFAULT '{}';
ALTER TABLE children DROP COLUMN IF EXISTS goal_ids;

-- The fixed-theme plan of 000013, empty: plans are made again when asked for.
CREATE TABLE IF NOT EXISTS plan_themes (
    day        SMALLINT PRIMARY KEY,
    title_uz   TEXT        NOT NULL,
    title_ru   TEXT        NOT NULL,
    icon       TEXT        NOT NULL,
    goal_id    UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT plan_themes_day_chk CHECK (day BETWEEN 1 AND 7)
);

INSERT INTO plan_themes (day, title_uz, title_ru, icon, goal_id)
SELECT t.day, t.title_uz, t.title_ru, t.icon,
       (SELECT g.id FROM goals g WHERE LOWER(g.name_en) = LOWER(t.goal_en) AND g.deleted_at IS NULL
        ORDER BY g.created_at, g.id LIMIT 1)
FROM (VALUES
    (1, 'Hisni top',                   'Угадай эмоцию',   'heart',   'Emotions'),
    (2, 'Meni tingla',                 'Послушай меня',   'chat',    'Language'),
    (3, 'Birga quramiz',               'Строим вместе',   'blocks',  'Motor skills'),
    (4, 'Kichik yaxshilik',            'Маленькое добро', 'gift',    'Social skills'),
    (5, 'Yangi narsani sinab ko‘ramiz', 'Пробуем новое',   'sparkle', NULL),
    (6, 'Birga o‘ylaymiz',             'Думаем вместе',   'bulb',    'Thinking'),
    (7, 'Bayram qilamiz',              'Празднуем',       'star',    'Emotions')
) AS t (day, title_uz, title_ru, icon, goal_en)
ON CONFLICT (day) DO NOTHING;

CREATE TABLE IF NOT EXISTS child_plan_days (
    child_id    UUID        NOT NULL REFERENCES children (id) ON DELETE CASCADE,
    day         SMALLINT    NOT NULL REFERENCES plan_themes (day),
    activity_id UUID        NOT NULL REFERENCES activities (id) ON DELETE CASCADE,
    started_on  DATE        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (child_id, day)
);
