
export interface Schedule {
    id: string
    user_id: string
    target_id: string
    start_time: string
    end_time: string
    recurrence_rule?: string
    timezone: string
    status: 'pending' | 'active' | 'expired' | 'cancelled'
    approval_status: 'pending' | 'approved' | 'rejected'
    rejection_reason?: string
    approved_by?: string
    approved_at?: string
    created_by?: string
    created_at: string
    updated_at: string
    metadata?: Record<string, any>
}
