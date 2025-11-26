-- Add role column to users table
ALTER TABLE users ADD COLUMN role VARCHAR(50) NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user', 'auditor'));

-- Add approval fields to schedules table
ALTER TABLE schedules ADD COLUMN approval_status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (approval_status IN ('pending', 'approved', 'rejected'));
ALTER TABLE schedules ADD COLUMN rejection_reason TEXT;
ALTER TABLE schedules ADD COLUMN approved_by UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE schedules ADD COLUMN approved_at TIMESTAMP WITH TIME ZONE;

-- Create index for approval status
CREATE INDEX idx_schedules_approval_status ON schedules(approval_status);
