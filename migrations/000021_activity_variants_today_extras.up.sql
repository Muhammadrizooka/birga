-- "Osonroq / Qiyinroq" variants of an activity (Pro, Figma 41); '' means none.
ALTER TABLE activities
    ADD COLUMN easier_uz TEXT NOT NULL DEFAULT '',
    ADD COLUMN easier_ru TEXT NOT NULL DEFAULT '',
    ADD COLUMN easier_en TEXT NOT NULL DEFAULT '',
    ADD COLUMN harder_uz TEXT NOT NULL DEFAULT '',
    ADD COLUMN harder_ru TEXT NOT NULL DEFAULT '',
    ADD COLUMN harder_en TEXT NOT NULL DEFAULT '';

-- Activities a Pro parent added to a child's list for one local day, next to the plan activity
-- (Figma 40a, 42). They are done through child_completions like any activity.
CREATE TABLE child_day_extras (
    child_id    UUID        NOT NULL REFERENCES children (id) ON DELETE CASCADE,
    day         DATE        NOT NULL,
    activity_id UUID        NOT NULL REFERENCES activities (id),
    added_at    TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (child_id, day, activity_id)
);
