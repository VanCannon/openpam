ALTER TABLE schedules ADD COLUMN IF NOT EXISTS provision_status TEXT DEFAULT 'pending';
