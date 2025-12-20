-- Update check_account_type constraint to use 'promotion' instead of 'user_promotion'
ALTER TABLE schedules DROP CONSTRAINT IF EXISTS check_account_type;
ALTER TABLE schedules ADD CONSTRAINT check_account_type CHECK (account_type IN ('promotion', 'managed', 'ephemeral', 'static'));
