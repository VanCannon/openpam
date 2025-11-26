-- Schedules table: Time-based access control
CREATE TABLE schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_id UUID NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE NOT NULL,
    recurrence_rule VARCHAR(500), -- iCal RRULE format
    timezone VARCHAR(100) DEFAULT 'UTC',
    status VARCHAR(50) NOT NULL CHECK (status IN ('pending', 'active', 'expired', 'cancelled')),
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB
);

CREATE INDEX idx_schedules_user_id ON schedules(user_id);
CREATE INDEX idx_schedules_target_id ON schedules(target_id);
CREATE INDEX idx_schedules_status ON schedules(status);
CREATE INDEX idx_schedules_time_range ON schedules(start_time, end_time);
