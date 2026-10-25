-- A child's age is now kept in years as a fraction (1 month = 1/12), so babies under one year
-- and half years (1.5, 2.5 ... 7) fit. The API speaks months; dbstore converts both ways.
ALTER TABLE children ALTER COLUMN age TYPE DOUBLE PRECISION USING age::double precision;
