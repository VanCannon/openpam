package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/openpam/scheduling/internal/schedule"
	"github.com/openpam/scheduling/pkg/logger"
)

type Publisher struct {
	nc     *nats.Conn
	logger *logger.Logger
}

func NewPublisher(natsURL string, log *logger.Logger) (*Publisher, error) {
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	log.Info("Connected to NATS", map[string]interface{}{
		"url": natsURL,
	})

	return &Publisher{
		nc:     nc,
		logger: log,
	}, nil
}

func (p *Publisher) Close() {
	if p.nc != nil {
		p.nc.Close()
	}
}

type ScheduleEvent struct {
	Type       string             `json:"type"`
	Schedule   *schedule.Schedule `json:"schedule"`
	Timestamp  time.Time          `json:"timestamp"`
	Message    string             `json:"message,omitempty"`
}

func (p *Publisher) PublishScheduleCreated(s *schedule.Schedule) error {
	event := &ScheduleEvent{
		Type:      "schedule.created",
		Schedule:  s,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := p.nc.Publish("openpam.schedule.created", data); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	p.logger.Debug("Published schedule.created event", map[string]interface{}{
		"schedule_id": s.ID,
	})

	return nil
}

func (p *Publisher) PublishScheduleActivated(s *schedule.Schedule) error {
	event := &ScheduleEvent{
		Type:      "schedule.activated",
		Schedule:  s,
		Timestamp: time.Now(),
		Message:   "Schedule is now active",
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := p.nc.Publish("openpam.schedule.activated", data); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	p.logger.Info("Published schedule.activated event", map[string]interface{}{
		"schedule_id": s.ID,
		"user_id":     s.UserID,
		"target_id":   s.TargetID,
	})

	return nil
}

func (p *Publisher) PublishScheduleExpired(s *schedule.Schedule) error {
	event := &ScheduleEvent{
		Type:      "schedule.expired",
		Schedule:  s,
		Timestamp: time.Now(),
		Message:   "Schedule has expired",
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := p.nc.Publish("openpam.schedule.expired", data); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	p.logger.Info("Published schedule.expired event", map[string]interface{}{
		"schedule_id": s.ID,
		"user_id":     s.UserID,
		"target_id":   s.TargetID,
	})

	return nil
}

func (p *Publisher) PublishScheduleUpdated(s *schedule.Schedule) error {
	event := &ScheduleEvent{
		Type:      "schedule.updated",
		Schedule:  s,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := p.nc.Publish("openpam.schedule.updated", data); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	p.logger.Debug("Published schedule.updated event", map[string]interface{}{
		"schedule_id": s.ID,
	})

	return nil
}

func (p *Publisher) PublishScheduleDeleted(scheduleID string) error {
	event := map[string]interface{}{
		"type":        "schedule.deleted",
		"schedule_id": scheduleID,
		"timestamp":   time.Now(),
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := p.nc.Publish("openpam.schedule.deleted", data); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	p.logger.Debug("Published schedule.deleted event", map[string]interface{}{
		"schedule_id": scheduleID,
	})

	return nil
}
