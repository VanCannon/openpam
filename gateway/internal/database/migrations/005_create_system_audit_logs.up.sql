-- System audit logs table: Records system events (logins, user changes, etc.)
CREATE TABLE system_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    event_type VARCHAR(100) NOT NULL, -- 'login_success', 'login_failed', 'user_created', 'user_updated', 'user_deleted', 'target_created', 'target_updated', 'target_deleted', 'credential_created', 'credential_updated', 'credential_deleted', 'session_started', 'session_ended', 'permission_changed', etc.
    user_id UUID REFERENCES users(id) ON DELETE SET NULL, -- The user who performed the action (NULL if system-initiated)
    target_user_id UUID REFERENCES users(id) ON DELETE SET NULL, -- The user who was affected by the action (e.g., the user that was created/updated/deleted)
    resource_type VARCHAR(100), -- 'user', 'target', 'credential', 'zone', 'session', etc.
    resource_id UUID, -- ID of the resource that was affected
    resource_name VARCHAR(255), -- Name/identifier of the resource for display
    action VARCHAR(100) NOT NULL, -- 'create', 'update', 'delete', 'login', 'logout', 'access', etc.
    status VARCHAR(50) NOT NULL, -- 'success', 'failure', 'pending'
    ip_address VARCHAR(45), -- Client IP address
    user_agent TEXT, -- Browser/client user agent
    details JSONB, -- Additional details about the event (what changed, error messages, etc.)
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_system_audit_logs_timestamp ON system_audit_logs(timestamp DESC);
CREATE INDEX idx_system_audit_logs_event_type ON system_audit_logs(event_type);
CREATE INDEX idx_system_audit_logs_user_id ON system_audit_logs(user_id);
CREATE INDEX idx_system_audit_logs_target_user_id ON system_audit_logs(target_user_id);
CREATE INDEX idx_system_audit_logs_resource_type ON system_audit_logs(resource_type);
CREATE INDEX idx_system_audit_logs_resource_id ON system_audit_logs(resource_id);
CREATE INDEX idx_system_audit_logs_status ON system_audit_logs(status);
