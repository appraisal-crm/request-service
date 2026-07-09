-- Client contact details captured at request creation.
-- Added nullable first, then promoted to NOT NULL so the migration is safe
-- against any pre-existing rows (must be backfilled before SET NOT NULL succeeds).
ALTER TABLE requests ADD COLUMN email        TEXT;
ALTER TABLE requests ADD COLUMN phone_number TEXT;

ALTER TABLE requests ALTER COLUMN email        SET NOT NULL;
ALTER TABLE requests ALTER COLUMN phone_number SET NOT NULL;
