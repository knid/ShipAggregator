package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/knid/kargonet/internal/api/middleware"
	"github.com/knid/kargonet/internal/domain"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/service/organization"
)

// LabelSettingsHandler handles label settings HTTP requests.
type LabelSettingsHandler struct {
	orgService *organization.Service
	logger     *observability.Logger
}

// NewLabelSettingsHandler creates a new label settings handler.
func NewLabelSettingsHandler(orgService *organization.Service, logger *observability.Logger) *LabelSettingsHandler {
	return &LabelSettingsHandler{
		orgService: orgService,
		logger:     logger,
	}
}

// LabelSettingsResponse is the API response for label settings.
type LabelSettingsResponse struct {
	Format     string  `json:"format"`
	Size       string  `json:"size"`
	IncludeLogo bool   `json:"include_logo"`
	LogoURL    *string `json:"logo_url"`
}

// UpdateLabelSettingsRequest is the request body for updating label settings.
type UpdateLabelSettingsRequest struct {
	Format      *string `json:"format,omitempty"`
	Size        *string `json:"size,omitempty"`
	IncludeLogo *bool   `json:"include_logo,omitempty"`
}

var validLabelFormats = map[string]bool{"pdf": true, "png": true, "zpl": true}
var validLabelSizes = map[string]bool{"a6": true, "a4": true, "10x15cm": true}

// GetLabelSettings handles GET /v1/label-settings
func (h *LabelSettingsHandler) GetLabelSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	org, err := h.orgService.GetOrganization(ctx, orgID)
	if err != nil {
		if errors.Is(err, domain.ErrOrganizationNotFound) {
			WriteError(w, r, NewNotFoundError("Organization"))
			return
		}
		h.logger.Error("Failed to get organization for label settings", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to get label settings"))
		return
	}

	writeJSON(w, http.StatusOK, settingsToLabelResponse(&org.Settings))
}

// UpdateLabelSettings handles PATCH /v1/label-settings
func (h *LabelSettingsHandler) UpdateLabelSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	var req UpdateLabelSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate format
	if req.Format != nil && !validLabelFormats[*req.Format] {
		WriteError(w, r, NewValidationError("format must be one of: pdf, png, zpl", nil))
		return
	}

	// Validate size
	if req.Size != nil && !validLabelSizes[*req.Size] {
		WriteError(w, r, NewValidationError("size must be one of: a6, a4, 10x15cm", nil))
		return
	}

	// Get current org to merge settings
	org, err := h.orgService.GetOrganization(ctx, orgID)
	if err != nil {
		if errors.Is(err, domain.ErrOrganizationNotFound) {
			WriteError(w, r, NewNotFoundError("Organization"))
			return
		}
		h.logger.Error("Failed to get organization for label settings update", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to update label settings"))
		return
	}

	// Merge label fields into existing settings
	settings := org.Settings
	if req.Format != nil {
		settings.DefaultLabelFormat = *req.Format
	}
	if req.Size != nil {
		settings.DefaultLabelSize = *req.Size
	}
	if req.IncludeLogo != nil {
		settings.LabelIncludeLogo = req.IncludeLogo
	}

	updated, err := h.orgService.UpdateOrganization(ctx, organization.UpdateOrganizationRequest{
		ID:       orgID,
		Settings: &settings,
	})
	if err != nil {
		h.logger.Error("Failed to update label settings", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to update label settings"))
		return
	}

	writeJSON(w, http.StatusOK, settingsToLabelResponse(&updated.Settings))
}

// UploadLogo handles POST /v1/label-settings/logo (stub — file storage not in scope)
func (h *LabelSettingsHandler) UploadLogo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"type":"about:blank","title":"Not Implemented","status":501,"detail":"Label logo upload is not yet implemented"}`))
}

// DeleteLogo handles DELETE /v1/label-settings/logo (stub)
func (h *LabelSettingsHandler) DeleteLogo(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func settingsToLabelResponse(s *domain.OrganizationSettings) LabelSettingsResponse {
	resp := LabelSettingsResponse{
		Format:  s.DefaultLabelFormat,
		Size:    s.DefaultLabelSize,
		LogoURL: s.LabelLogoURL,
	}

	// Defaults
	if resp.Format == "" {
		resp.Format = "pdf"
	}
	if resp.Size == "" {
		resp.Size = "a6"
	}

	if s.LabelIncludeLogo != nil {
		resp.IncludeLogo = *s.LabelIncludeLogo
	}

	return resp
}
