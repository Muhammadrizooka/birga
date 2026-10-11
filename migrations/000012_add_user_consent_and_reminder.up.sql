-- Consent to the terms, given with the tick on the sign-up screen. FALSE for users who signed up
-- before the app sent it, or were created by an admin.
ALTER TABLE users ADD COLUMN IF NOT EXISTS agreed_on_conditions BOOLEAN NOT NULL DEFAULT FALSE;

-- The daily reminder setting from the profile page: on or off, and the local time (Uzbekistan,
-- HH:MM, 24-hour) the app should remind at. The app schedules the notification itself.
ALTER TABLE users ADD COLUMN IF NOT EXISTS reminder_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS reminder_time VARCHAR(5) NOT NULL DEFAULT '18:00';

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'users_reminder_time_chk') THEN
        ALTER TABLE users ADD CONSTRAINT users_reminder_time_chk
            CHECK (reminder_time ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$');
    END IF;
END $$;
