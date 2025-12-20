package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/middleware"
	"github.com/VanCannon/openpam/gateway/internal/sse"
	"github.com/google/uuid"
)

// SSEHandler handles Server-Sent Events for real-time updates
type SSEHandler struct {
	broadcaster *sse.Broadcaster
	logger      *logger.Logger
}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler(broadcaster *sse.Broadcaster, log *logger.Logger) *SSEHandler {
	return &SSEHandler{
		broadcaster: broadcaster,
		logger:      log,
	}
}

// HandleScheduleUpdates streams schedule updates to the client via SSE
func (h *SSEHandler) HandleScheduleUpdates() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user ID from context (set by auth middleware)
		userID := middleware.GetUserID(r.Context())
		if userID == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Set headers for SSE
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("X-Accel-Buffering", "no") // Disable proxy buffering

		// Get origin from request
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "http://localhost:3000" // Default for local development
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)

		// Get ResponseController to manage deadlines for long-lived connection
		rc := http.NewResponseController(w)

		// Create client ID
		clientID := uuid.New().String()

		// Add client to broadcaster
		client := h.broadcaster.AddClient(clientID, userID)
		defer h.broadcaster.RemoveClient(clientID)

		h.logger.Info("SSE client connected", map[string]interface{}{
			"client_id": clientID,
			"user_id":   userID,
		})

		// Helper function to write and flush with deadline extension
		writeAndFlush := func(data string) error {
			// Extend write deadline before each write (30 seconds should be plenty)
			if err := rc.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
				h.logger.Error("Failed to extend write deadline", map[string]interface{}{
					"error": err.Error(),
				})
				// Continue anyway - older Go versions may not support this
			}

			if _, err := fmt.Fprint(w, data); err != nil {
				return err
			}

			if err := rc.Flush(); err != nil {
				return err
			}

			return nil
		}

		// Send initial connection confirmation
		if err := writeAndFlush(fmt.Sprintf("data: {\"type\":\"connected\",\"timestamp\":\"%s\"}\n\n", time.Now().Format(time.RFC3339))); err != nil {
			h.logger.Error("Failed to send initial SSE message", map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		// Create ticker for keepalive (every 20 seconds)
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()

		h.logger.Info("Starting SSE event loop", map[string]interface{}{
			"client_id": clientID,
		})

		// Stream events to client
		for {
			select {
			case <-r.Context().Done():
				// Client disconnected
				h.logger.Info("SSE client disconnected", map[string]interface{}{
					"client_id": clientID,
					"user_id":   userID,
				})
				return

			case message, ok := <-client.Channel:
				if !ok {
					h.logger.Info("SSE client channel closed", map[string]interface{}{
						"client_id": clientID,
					})
					return
				}

				// Write message to client with deadline extension
				if err := writeAndFlush(string(message)); err != nil {
					h.logger.Error("Failed to write SSE message", map[string]interface{}{
						"error": err.Error(),
					})
					return
				}

			case <-ticker.C:
				// Send keepalive ping with deadline extension
				if err := writeAndFlush(": keepalive\n\n"); err != nil {
					h.logger.Debug("Failed to send keepalive (client likely disconnected)", map[string]interface{}{
						"error": err.Error(),
					})
					return
				}
			}
		}
	}
}
