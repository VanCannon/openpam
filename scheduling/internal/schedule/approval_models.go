package schedule

// ApproveScheduleRequest represents a request to approve a schedule
type ApproveScheduleRequest struct {
	ScheduleID string  `json:"schedule_id"`
	ApprovedBy string  `json:"approved_by"`
	StartTime  *string `json:"start_time,omitempty"` // Optional: modify start time
	EndTime    *string `json:"end_time,omitempty"`   // Optional: modify end time
}

// RejectScheduleRequest represents a request to reject a schedule
type RejectScheduleRequest struct {
	ScheduleID string `json:"schedule_id"`
	RejectedBy string `json:"rejected_by"`
	Reason     string `json:"reason"`
}
