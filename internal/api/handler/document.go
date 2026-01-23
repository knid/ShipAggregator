// Package handler provides HTTP handlers for the API.
package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/knid/kargonet/internal/api/middleware"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/repository/postgres/db"
)

// DocumentHandler handles document-related HTTP requests.
type DocumentHandler struct {
	queries *db.Queries
	logger  *observability.Logger
}

// NewDocumentHandler creates a new document handler.
func NewDocumentHandler(queries *db.Queries, logger *observability.Logger) *DocumentHandler {
	return &DocumentHandler{
		queries: queries,
		logger:  logger,
	}
}

// GetDocument handles GET /documents/{id}
// Returns document metadata.
func (h *DocumentHandler) GetDocument(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get document ID from URL
	documentIDStr := chi.URLParam(r, "id")
	documentID, err := uuid.Parse(documentIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid document ID"))
		return
	}

	// Get document
	doc, err := h.queries.GetDocumentByID(ctx, documentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Document"))
			return
		}
		h.logger.Error("Failed to get document", "error", err, "document_id", documentID)
		WriteError(w, r, NewInternalError("Failed to get document"))
		return
	}

	// Verify organization ownership
	if doc.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Document"))
		return
	}

	writeJSON(w, http.StatusOK, documentToResponse(doc))
}

// ListDocuments handles GET /documents
// Returns documents for the current organization.
func (h *DocumentHandler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Parse query parameters
	docType := r.URL.Query().Get("type")

	// Get documents
	docs, err := h.queries.ListDocumentsByOrganization(ctx, db.ListDocumentsByOrganizationParams{
		OrganizationID: orgID,
		Type:           &docType,
	})
	if err != nil {
		h.logger.Error("Failed to list documents", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to list documents"))
		return
	}

	response := make([]DocumentResponse, len(docs))
	for i, doc := range docs {
		response[i] = documentToResponse(doc)
	}

	writeJSON(w, http.StatusOK, response)
}

// DownloadDocument handles GET /documents/{id}/download
// Returns the document content.
func (h *DocumentHandler) DownloadDocument(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get document ID from URL
	documentIDStr := chi.URLParam(r, "id")
	documentID, err := uuid.Parse(documentIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid document ID"))
		return
	}

	// Get document
	doc, err := h.queries.GetDocumentByID(ctx, documentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Document"))
			return
		}
		h.logger.Error("Failed to get document", "error", err, "document_id", documentID)
		WriteError(w, r, NewInternalError("Failed to get document"))
		return
	}

	// Verify organization ownership
	if doc.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Document"))
		return
	}

	// TODO: Implement actual file retrieval from storage (S3, GCS, etc.)
	// For now, return a redirect URL or the document metadata
	// In production, this would:
	// 1. Generate a pre-signed URL for the storage backend
	// 2. Return a redirect to that URL
	// 3. Or stream the file content directly

	// For now, return the document metadata with a placeholder download URL
	response := DocumentDownloadResponse{
		Document:    documentToResponse(doc),
		DownloadURL: "/storage/" + doc.StorageBucket + "/" + doc.StoragePath,
		ExpiresAt:   "", // Would be set for pre-signed URLs
	}

	writeJSON(w, http.StatusOK, response)
}

// Response types

// DocumentResponse represents a document in API responses.
type DocumentResponse struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Type           string    `json:"type"`
	Format         string    `json:"format"`
	Filename       string    `json:"filename"`
	ContentType    string    `json:"content_type"`
	SizeBytes      int32     `json:"size_bytes"`
	ContentHash    string    `json:"content_hash,omitempty"`
	CreatedAt      string    `json:"created_at"`
}

// DocumentDownloadResponse represents a document download response.
type DocumentDownloadResponse struct {
	Document    DocumentResponse `json:"document"`
	DownloadURL string           `json:"download_url"`
	ExpiresAt   string           `json:"expires_at,omitempty"`
}

// Conversion helpers

func documentToResponse(doc *db.Document) DocumentResponse {
	return DocumentResponse{
		ID:             doc.ID,
		OrganizationID: doc.OrganizationID,
		Type:           doc.Type,
		Format:         doc.Format,
		Filename:       doc.Filename,
		ContentType:    doc.ContentType,
		SizeBytes:      doc.SizeBytes,
		ContentHash:    doc.ContentHash,
		CreatedAt:      doc.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
}
