-- Difficulty level of an activity: the "Osonroq / Odatdagi / Qiyinroq" switch on the activity page
-- swaps the activity for another one of the chosen level. Existing activities are normal.
ALTER TABLE activities
    ADD COLUMN difficulty TEXT NOT NULL DEFAULT 'normal'
        CONSTRAINT activities_difficulty_chk CHECK (difficulty IN ('easy', 'normal', 'difficult'));
