-- Revert to old constraint
ALTER TABLE schedules DROP CONSTRAINT IF EXISTS check_account_type;
ALTER TABLE schedules ADD CONSTRAINT check_account_type CHECK (account_type IN ('user_promotion', 'managed', 'ephemeral', 'static'));
