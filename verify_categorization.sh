#!/bin/bash

# 1. Login as Admin
curl -s "http://localhost:8080/api/v1/auth/login?role=admin" -c admin_cookies.txt
ADMIN_UUID=$(curl -s "http://localhost:8080/api/v1/auth/me" -b admin_cookies.txt | jq -r '.id')
TARGET_ID=$(curl -s "http://localhost:8080/api/v1/targets" -b admin_cookies.txt | jq -r '.targets[0].id')
JOHN_UUID="42784e0d-155a-4c6c-8ebc-fca846403b64"

echo "1. Creating Standing Request for John..."
# Note: Admin auto-approves their own requests, so we must be careful.
# But here we want to simulate a user request. Since we can't easily login as John (no password),
# we'll have Admin create it for John. Admin-created requests are auto-approved.
# Wait, if Admin creates it, it's auto-approved immediately.
# We need to test the "Approve with modification" flow.
# So we need a PENDING request.
# Admin can create a request for John. Does it get auto-approved?
# Code says: if userRole == models.RoleAdmin { schedule.ApprovalStatus = models.ApprovalStatusApproved ... }
# So yes, it gets auto-approved.

# Workaround: Manually insert a pending schedule into DB or use a non-admin user if possible.
# Or, modify the code temporarily to disable auto-approval? No.
# Let's use the "dev-user-123" if we can login as them.
# The dev login endpoint allows logging in as any role.
curl -s "http://localhost:8080/api/v1/auth/login?role=user&email=dev@example.com" -c user_cookies.txt
USER_UUID=$(curl -s "http://localhost:8080/api/v1/auth/me" -b user_cookies.txt | jq -r '.id')

echo "Creating Standing Request as User..."
curl -s "http://localhost:8080/api/v1/schedules/request" \
  -X POST \
  -H "Content-Type: application/json" \
  -b user_cookies.txt \
  -d '{
    "target_id": "'$TARGET_ID'",
    "type": "standing",
    "account_type": "static",
    "user_id": "'$USER_UUID'",
    "start_time": "2025-01-01T00:00:00Z",
    "end_time": "2025-01-01T01:00:00Z",
    "timezone": "UTC"
  }' > request_response.json
SCHEDULE_ID=$(jq -r '.schedule.id' request_response.json)
echo "Schedule ID: $SCHEDULE_ID"

# Verify it is pending and standing
echo "Verifying Initial State..."
curl -s "http://localhost:8080/api/v1/schedules?user_id=$USER_UUID" -b admin_cookies.txt > initial_state.json
jq -r '.schedules[] | select(.id=="'$SCHEDULE_ID'") | "Type: " + .type + ", Status: " + .approval_status' initial_state.json

# 2. Approve with Time Change (Admin)
echo "Approving with Time Change..."
# Set time to NOW so it becomes Active
START_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
END_TIME=$(date -u -d "+1 hour" +"%Y-%m-%dT%H:%M:%SZ")

curl -s "http://localhost:8080/api/v1/schedules/approve" \
  -X POST \
  -H "Content-Type: application/json" \
  -b admin_cookies.txt \
  -d '{
    "schedule_id": "'$SCHEDULE_ID'",
    "start_time": "'$START_TIME'",
    "end_time": "'$END_TIME'"
  }' > approve_response.json

# 3. Verify Final State
echo "Verifying Final State..."
curl -s "http://localhost:8080/api/v1/schedules?user_id=$USER_UUID" -b admin_cookies.txt > final_state.json
jq -r '.schedules[] | select(.id=="'$SCHEDULE_ID'") | "Type: " + .type + ", Status: " + .approval_status + ", Start: " + .start_time' final_state.json

