-- English as the third language for activities. Existing rows get '' until an admin fills them
-- in; the API requires both fields on create and refuses to clear them on update.
ALTER TABLE activities
    ADD COLUMN IF NOT EXISTS title_en       TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS description_en TEXT NOT NULL DEFAULT '';
