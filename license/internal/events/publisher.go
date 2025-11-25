package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/openpam/license/pkg/logger"
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

type LicenseValidationEvent struct {
	Type         string                 `json:"type"`
	LicenseKey   string                 `json:"license_key"`
	Valid        bool                   `json:"valid"`
	Errors       []string               `json:"errors,omitempty"`
	Features     map[string]interface{} `json:"features"`
	Timestamp    time.Time              `json:"timestamp"`
	RemainingDays *int                  `json:"remaining_days,omitempty"`
}

type UsageThresholdEvent struct {
	Type       string    `json:"type"`
	Resource   string    `json:"resource"`
	Current    int       `json:"current"`
	Limit      int       `json:"limit"`
	Percentage float64   `json:"percentage"`
	Timestamp  time.Time `json:"timestamp"`
}

type FeatureAccessEvent struct {
	Type      string    `json:"type"`
	Feature   string    `json:"feature"`
	Enabled   bool      `json:"enabled"`
	UserID    string    `json:"user_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func (p *Publisher) PublishLicenseValidation(event *LicenseValidationEvent) error {
	event.Timestamp = time.Now()
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := p.nc.Publish("openpam.license.validation", data); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	p.logger.Debug("Published license validation event", map[string]interface{}{
		"valid": event.Valid,
	})

	return nil
}

func (p *Publisher) PublishUsageThreshold(event *UsageThresholdEvent) error {
	event.Timestamp = time.Now()
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := p.nc.Publish("openpam.license.threshold", data); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	p.logger.Warn("Published usage threshold event", map[string]interface{}{
		"resource":   event.Resource,
		"percentage": event.Percentage,
	})

	return nil
}

func (p *Publisher) PublishFeatureAccess(event *FeatureAccessEvent) error {
	event.Timestamp = time.Now()
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := p.nc.Publish("openpam.license.feature", data); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	p.logger.Debug("Published feature access event", map[string]interface{}{
		"feature": event.Feature,
		"enabled": event.Enabled,
	})

	return nil
}
