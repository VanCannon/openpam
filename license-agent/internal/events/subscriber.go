package events

import (
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/openpam/license-agent/internal/license"
	"github.com/openpam/license-agent/pkg/logger"
)

type Subscriber struct {
	nc      *nats.Conn
	service *license.Service
	logger  *logger.Logger
	subs    []*nats.Subscription
}

func NewSubscriber(natsURL string, service *license.Service, log *logger.Logger) (*Subscriber, error) {
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	log.Info("Subscriber connected to NATS", map[string]interface{}{
		"url": natsURL,
	})

	return &Subscriber{
		nc:      nc,
		service: service,
		logger:  log,
		subs:    make([]*nats.Subscription, 0),
	}, nil
}

func (s *Subscriber) Start() error {
	// Subscribe to session events to track concurrent sessions
	sub1, err := s.nc.Subscribe("openpam.session.started", s.handleSessionStarted)
	if err != nil {
		return fmt.Errorf("failed to subscribe to session.started: %w", err)
	}
	s.subs = append(s.subs, sub1)

	sub2, err := s.nc.Subscribe("openpam.session.ended", s.handleSessionEnded)
	if err != nil {
		return fmt.Errorf("failed to subscribe to session.ended: %w", err)
	}
	s.subs = append(s.subs, sub2)

	s.logger.Info("Subscribed to NATS topics", map[string]interface{}{
		"topics": []string{"openpam.session.started", "openpam.session.ended"},
	})

	return nil
}

func (s *Subscriber) handleSessionStarted(msg *nats.Msg) {
	var event struct {
		SessionID string `json:"session_id"`
		UserID    string `json:"user_id"`
		TargetID  string `json:"target_id"`
	}

	if err := json.Unmarshal(msg.Data, &event); err != nil {
		s.logger.Error("Failed to unmarshal session.started event", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	s.logger.Debug("Session started", map[string]interface{}{
		"session_id": event.SessionID,
		"user_id":    event.UserID,
	})

	// Check if session limit is exceeded
	stats, err := s.service.GetUsageStats()
	if err != nil {
		s.logger.Error("Failed to get usage stats", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	license, err := s.service.GetActiveLicense()
	if err != nil {
		s.logger.Error("Failed to get active license", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	if license.MaxSessions != nil && stats.CurrentSessions > *license.MaxSessions {
		s.logger.Warn("Session limit exceeded", map[string]interface{}{
			"current": stats.CurrentSessions,
			"limit":   *license.MaxSessions,
		})
		// Could publish an alert event here
	}
}

func (s *Subscriber) handleSessionEnded(msg *nats.Msg) {
	var event struct {
		SessionID string `json:"session_id"`
		UserID    string `json:"user_id"`
	}

	if err := json.Unmarshal(msg.Data, &event); err != nil {
		s.logger.Error("Failed to unmarshal session.ended event", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	s.logger.Debug("Session ended", map[string]interface{}{
		"session_id": event.SessionID,
	})
}

func (s *Subscriber) Close() {
	for _, sub := range s.subs {
		sub.Unsubscribe()
	}
	if s.nc != nil {
		s.nc.Close()
	}
}
