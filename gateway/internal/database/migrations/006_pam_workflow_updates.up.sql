CREATE TABLE IF NOT EXISTS groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    source VARCHAR(50) NOT NULL CHECK (source IN ('local', 'ad')),
    ad_guid VARCHAR(255),
    dn VARCHAR(500), -- Distinguished Name for AD groups
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_groups (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, group_id)
);

ALTER TABLE schedules ADD COLUMN IF NOT EXISTS type VARCHAR(50) NOT NULL DEFAULT 'scheduled';
ALTER TABLE schedules ADD CONSTRAINT check_type CHECK (type IN ('scheduled', 'standing'));
ALTER TABLE schedules ADD COLUMN IF NOT EXISTS account_type VARCHAR(50) NOT NULL DEFAULT 'static';
ALTER TABLE schedules ADD CONSTRAINT check_account_type CHECK (account_type IN ('user_promotion', 'managed', 'ephemeral', 'static'));
ALTER TABLE schedules ADD COLUMN IF NOT EXISTS account_details JSONB;
