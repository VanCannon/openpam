ALTER TABLE users ADD COLUMN IF NOT EXISTS source VARCHAR(50) NOT NULL DEFAULT 'local' CHECK (source IN ('local', 'active_directory'));
