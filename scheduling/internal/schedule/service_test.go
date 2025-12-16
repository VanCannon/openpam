package schedule

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/VanCannon/openpam/scheduling/pkg/logger"
)

func TestCreateSchedule(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	log := logger.New("info", "text")
	service := NewService(db, log)

	req := &CreateScheduleRequest{
		UserID:      "user-123",
		TargetID:    "target-456",
		StartTime:   time.Now(),
		EndTime:     time.Now().Add(1 * time.Hour),
		Type:        "standing",
		AccountType: "ephemeral",
		AccountDetails: map[string]interface{}{
			"username": "temp-user",
		},
	}

	mock.ExpectExec("INSERT INTO schedules").
		WithArgs(
			sqlmock.AnyArg(), // id
			req.UserID,
			req.TargetID,
			req.StartTime,
			req.EndTime,
			req.RecurrenceRule,
			req.Timezone,
			"pending",
			"pending",
			sqlmock.AnyArg(), // created_by
			sqlmock.AnyArg(), // created_at
			sqlmock.AnyArg(), // updated_at
			sqlmock.AnyArg(), // metadata
			req.Type,
			req.AccountType,
			sqlmock.AnyArg(), // account_details
			"pending",        // provision_status
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	schedule, err := service.CreateSchedule(req, "creator-789")
	if err != nil {
		t.Errorf("error was not expected while updating stats: %s", err)
	}

	if schedule.Type != req.Type {
		t.Errorf("expected type %s, got %s", req.Type, schedule.Type)
	}
	if schedule.AccountType != req.AccountType {
		t.Errorf("expected account type %s, got %s", req.AccountType, schedule.AccountType)
	}
	if schedule.ProvisionStatus != "pending" {
		t.Errorf("expected provision status pending, got %s", schedule.ProvisionStatus)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestGetSchedule(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	log := logger.New("info", "text")
	service := NewService(db, log)

	id := "schedule-123"
	userID := "user-123"
	targetID := "target-456"
	startTime := time.Now()
	endTime := time.Now().Add(1 * time.Hour)
	scheduleType := "standing"
	accountType := "ephemeral"
	provisionStatus := "provisioned"
	accountDetails := map[string]interface{}{"username": "temp-user"}
	accountDetailsJSON, _ := json.Marshal(accountDetails)

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "target_id", "start_time", "end_time", "recurrence_rule",
		"timezone", "status", "approval_status", "rejection_reason", "approved_by", "approved_at",
		"created_by", "created_at", "updated_at", "metadata", "type", "account_type", "account_details", "provision_status",
	}).AddRow(
		id, userID, targetID, startTime, endTime, nil,
		"UTC", "active", "approved", nil, "approver-1", time.Now(),
		"creator-1", time.Now(), time.Now(), nil, scheduleType, accountType, accountDetailsJSON, provisionStatus,
	)

	mock.ExpectQuery("SELECT .* FROM schedules WHERE id = \\$1").
		WithArgs(id).
		WillReturnRows(rows)

	schedule, err := service.GetSchedule(id)
	if err != nil {
		t.Errorf("error was not expected while getting schedule: %s", err)
	}

	if schedule.ID != id {
		t.Errorf("expected id %s, got %s", id, schedule.ID)
	}
	if schedule.Type != scheduleType {
		t.Errorf("expected type %s, got %s", scheduleType, schedule.Type)
	}
	if schedule.AccountType != accountType {
		t.Errorf("expected account type %s, got %s", accountType, schedule.AccountType)
	}
	if schedule.ProvisionStatus != provisionStatus {
		t.Errorf("expected provision status %s, got %s", provisionStatus, schedule.ProvisionStatus)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
