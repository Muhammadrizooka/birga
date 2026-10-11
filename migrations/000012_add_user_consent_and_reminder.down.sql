ALTER TABLE users DROP CONSTRAINT IF EXISTS users_reminder_time_chk;
ALTER TABLE users DROP COLUMN IF EXISTS reminder_time;
ALTER TABLE users DROP COLUMN IF EXISTS reminder_enabled;
ALTER TABLE users DROP COLUMN IF EXISTS agreed_on_conditions;
