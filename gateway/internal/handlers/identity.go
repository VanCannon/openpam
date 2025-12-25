package handlers

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/middleware"
	"github.com/VanCannon/openpam/gateway/internal/models"
	"github.com/VanCannon/openpam/gateway/internal/repository"
	"github.com/google/uuid"
)

// IdentityHandler handles requests related to identity management
type IdentityHandler struct {
	identityURL     string
	orchestratorURL string
	systemAuditRepo *repository.SystemAuditLogRepository
	logger          *logger.Logger
}

// NewIdentityHandler creates a new IdentityHandler
func NewIdentityHandler(identityURL, orchestratorURL string, systemAuditRepo *repository.SystemAuditLogRepository, logger *logger.Logger) *IdentityHandler {
	return &IdentityHandler{
		identityURL:     identityURL,
		orchestratorURL: orchestratorURL,
		systemAuditRepo: systemAuditRepo,
		logger:          logger,
	}
}

// proxyRequest proxies the request to the target URL
func (h *IdentityHandler) proxyRequest(targetURL string, w http.ResponseWriter, r *http.Request) {
	target, err := url.Parse(targetURL)
	if err != nil {
		h.logger.Error("Failed to parse target URL", map[string]interface{}{
			"url":   targetURL,
			"error": err.Error(),
		})
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Update the request URL path to match the target service's expected path
	// The gateway receives /api/v1/..., and the services also expect /api/v1/...
	// So we just need to set the host and scheme
	r.URL.Host = target.Host
	r.URL.Scheme = target.Scheme
	r.Host = target.Host

	// Remove authorization header if it's a bearer token for the gateway
	// But wait, the services might need auth?
	// For now, assuming internal services don't require gateway token auth,
	// or if they do, we might need to handle it.
	// The current implementation of Identity Service doesn't seem to check auth in the handler code we saw.

	proxy.ServeHTTP(w, r)
}

// HandleGetConfig proxies the request to get AD config
func (h *IdentityHandler) HandleGetConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.proxyRequest(h.identityURL, w, r)
	}
}

// HandleSaveConfig proxies the request to save AD config
func (h *IdentityHandler) HandleSaveConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.proxyRequest(h.identityURL, w, r)
	}
}

// HandleListADUsers proxies the request to list AD users
func (h *IdentityHandler) HandleListADUsers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.proxyRequest(h.identityURL, w, r)
	}
}

// HandleListADComputers proxies the request to list AD computers
func (h *IdentityHandler) HandleListADComputers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.proxyRequest(h.identityURL, w, r)
	}
}

// HandleListADGroups proxies the request to list AD groups
func (h *IdentityHandler) HandleListADGroups() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.proxyRequest(h.identityURL, w, r)
	}
}

// HandleImportUser proxies the request to import an AD user
func (h *IdentityHandler) HandleImportUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Audit log
		ctx := r.Context()
		actorIDStr := middleware.GetUserID(ctx)
		var actorID *uuid.UUID
		if uid, err := uuid.Parse(actorIDStr); err == nil {
			actorID = &uid
		}

		h.systemAuditRepo.CreateSimple(ctx, models.EventTypeUserCreated, actorID, "import_user", models.AuditStatusSuccess, nil, map[string]interface{}{
			"source": "ad_import",
		})
		h.proxyRequest(h.identityURL, w, r)
	}
}

// HandleImportGroup proxies the request to import an AD group
func (h *IdentityHandler) HandleImportGroup() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Audit log
		ctx := r.Context()
		actorIDStr := middleware.GetUserID(ctx)
		var actorID *uuid.UUID
		if uid, err := uuid.Parse(actorIDStr); err == nil {
			actorID = &uid
		}

		h.systemAuditRepo.CreateSimple(ctx, models.EventTypeGroupCreated, actorID, "import_group", models.AuditStatusSuccess, nil, map[string]interface{}{
			"source": "ad_import",
		})
		h.proxyRequest(h.identityURL, w, r)
	}
}

// HandleImportComputer proxies the request to import an AD computer
func (h *IdentityHandler) HandleImportComputer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Audit log
		ctx := r.Context()
		actorIDStr := middleware.GetUserID(ctx)
		var actorID *uuid.UUID
		if uid, err := uuid.Parse(actorIDStr); err == nil {
			actorID = &uid
		}

		h.systemAuditRepo.CreateSimple(ctx, models.EventTypeTargetCreated, actorID, "import_computer", models.AuditStatusSuccess, nil, map[string]interface{}{
			"source": "ad_import",
		})
		h.proxyRequest(h.identityURL, w, r)
	}
}

// HandleSyncAD proxies the request to trigger AD sync via Orchestrator
func (h *IdentityHandler) HandleSyncAD() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Audit log
		ctx := r.Context()
		actorIDStr := middleware.GetUserID(ctx)
		var actorID *uuid.UUID
		if uid, err := uuid.Parse(actorIDStr); err == nil {
			actorID = &uid
		}

		h.systemAuditRepo.CreateSimple(ctx, "ad_sync", actorID, "trigger_ad_sync", models.AuditStatusSuccess, nil, nil)
		h.proxyRequest(h.orchestratorURL, w, r)
	}
}

// HandleListManagedAccounts proxies the request to list managed accounts
func (h *IdentityHandler) HandleListManagedAccounts() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.proxyRequest(h.identityURL, w, r)
	}
}

// HandleDeleteManagedAccount proxies the request to delete managed accounts
func (h *IdentityHandler) HandleDeleteManagedAccount() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Audit log
		ctx := r.Context()
		actorIDStr := middleware.GetUserID(ctx)
		var actorID *uuid.UUID
		if uid, err := uuid.Parse(actorIDStr); err == nil {
			actorID = &uid
		}

		id := r.URL.Path[len("/api/v1/managed-accounts/"):]

		h.systemAuditRepo.CreateSimple(ctx, models.EventTypeCredentialDeleted, actorID, "delete_managed_account", models.AuditStatusSuccess, nil, map[string]interface{}{
			"managed_account_id": id,
		})
		h.proxyRequest(h.identityURL, w, r)
	}
}
