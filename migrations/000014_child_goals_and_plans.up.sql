-- Goals move from the parent to the child: a child has its own goals, picked when it is added.
-- Each child starts with the goals its first parent picked at sign-up; users.goal_ids then goes.
ALTER TABLE children ADD COLUMN IF NOT EXISTS goal_ids UUID[] NOT NULL DEFAULT '{}';

UPDATE children c SET goal_ids = u.goal_ids
FROM (
    SELECT DISTINCT ON (uc.child_id) uc.child_id, u.goal_ids
    FROM user_children uc JOIN users u ON u.id = uc.user_id
    ORDER BY uc.child_id, uc.created_at, u.id
) u
WHERE u.child_id = c.id AND c.goal_ids = '{}';

ALTER TABLE users DROP COLUMN IF EXISTS goal_ids;

-- The plan no longer follows seven fixed themes: each day serves one of the child's goals. The
-- old plans are dropped; every child gets a new plan the next time it is asked for.
DROP TABLE IF EXISTS child_plan_days;
DROP TABLE IF EXISTS plan_themes;

-- One row per 7-day plan of a child. number counts the child's plans from 1; the latest is the
-- current one. started_on is the local day the plan was made.
CREATE TABLE IF NOT EXISTS child_plans (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    child_id   UUID        NOT NULL REFERENCES children (id) ON DELETE CASCADE,
    number     INT         NOT NULL,
    started_on DATE        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT child_plans_number_chk CHECK (number >= 1),
    CONSTRAINT child_plans_child_number_uniq UNIQUE (child_id, number)
);

-- The seven days of a plan. goal_id is the child's goal the day serves (NULL when the child had
-- no goals); like other goal_ids it is matched by id only. completed_on is the local day the
-- day's activity was done as part of the plan; NULL while the day is not done. These days are
-- what the child's streak counts.
CREATE TABLE IF NOT EXISTS child_plan_days (
    plan_id      UUID        NOT NULL REFERENCES child_plans (id) ON DELETE CASCADE,
    day          SMALLINT    NOT NULL,
    goal_id      UUID,
    activity_id  UUID        NOT NULL REFERENCES activities (id) ON DELETE CASCADE,
    completed_on DATE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (plan_id, day),
    CONSTRAINT child_plan_days_day_chk CHECK (day BETWEEN 1 AND 7)
);

-- Serves the streak: the done days of a child's plans.
CREATE INDEX IF NOT EXISTS child_plan_days_done_idx ON child_plan_days (plan_id, completed_on)
    WHERE completed_on IS NOT NULL;
