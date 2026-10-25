ALTER TABLE children ALTER COLUMN age TYPE SMALLINT USING round(age)::smallint;
