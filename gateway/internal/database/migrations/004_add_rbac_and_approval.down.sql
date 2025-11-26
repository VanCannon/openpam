-- Remove index
DROP INDEX IF EXISTS idx_schedules_approval_status;

-- Remove approval fields from schedules table
ALTER TABLE schedules DROP COLUMN IF EXISTS approved_at;
ALTER TABLE schedules DROP COLUMN IF EXISTS approved_by;
ALTER TABLE schedules DROP COLUMN IF EXISTS rejection_reason;
ALTER TABLE schedules DROP COLUMN IF EXISTS approval_status;

-- Remove role column from users table
ALTER TABLE users DROP COLUMN IF EXISTS role;
