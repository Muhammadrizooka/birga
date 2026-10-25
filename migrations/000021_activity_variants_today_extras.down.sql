DROP TABLE IF EXISTS child_day_extras;

ALTER TABLE activities
    DROP COLUMN IF EXISTS easier_uz,
    DROP COLUMN IF EXISTS easier_ru,
    DROP COLUMN IF EXISTS easier_en,
    DROP COLUMN IF EXISTS harder_uz,
    DROP COLUMN IF EXISTS harder_ru,
    DROP COLUMN IF EXISTS harder_en;
