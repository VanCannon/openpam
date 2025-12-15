ALTER TABLE groups DROP CONSTRAINT IF EXISTS groups_source_check;
ALTER TABLE groups ADD CONSTRAINT groups_source_check CHECK (source IN ('local', 'ad', 'active_directory'));
