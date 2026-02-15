package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/knid/kargonet/internal/domain"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/service/auth"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	sessionService *auth.SessionService
	logger         *observability.Logger
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(sessionService *auth.SessionService, logger *observability.Logger) *AuthHandler {
	return &AuthHandler{
		sessionService: sessionService,
		logger:         logger,
	}
}

// RegisterRequest represents the request body for registration.
type RegisterRequest struct {
	OrganizationName string `json:"organization_name"`
	Email            string `json:"email"`
	Name             string `json:"name"`
	Password         string `json:"password"`
}

// RegisterResponse represents the response for a successful registration.
type RegisterResponse struct {
	Token          string     `json:"session_token"`
	ExpiresAt      time.Time  `json:"expires_at"`
	Member         MemberInfo `json:"member"`
	Role           RoleInfo   `json:"role"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	SessionID      uuid.UUID  `json:"session_id"`
}

// LoginRequest represents the request body for login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse represents the response for a successful login.
type LoginResponse struct {
	Token     string     `json:"session_token"`
	ExpiresAt time.Time  `json:"expires_at"`
	Member    MemberInfo `json:"member"`
	Role      RoleInfo   `json:"role"`
	SessionID uuid.UUID  `json:"session_id"`
}

// MemberInfo represents member information in responses.
type MemberInfo struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	Email          string     `json:"email"`
	Name           string     `json:"name"`
	Status         string     `json:"status"`
	JoinedAt       *time.Time `json:"joined_at,omitempty"`
	LastLoginAt    *time.Time `json:"last_login_at,omitempty"`
}

// RoleInfo represents role information in responses.
type RoleInfo struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Permissions []string  `json:"permissions"`
	IsSystem    bool      `json:"is_system"`
}

// MeResponse represents the response for the /auth/me endpoint.
type MeResponse struct {
	Member       MemberInfo       `json:"member"`
	Role         RoleInfo         `json:"role"`
	Organization OrganizationInfo `json:"organization"`
	Session      SessionInfo      `json:"session"`
}

// OrganizationInfo represents organization information in responses.
type OrganizationInfo struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name,omitempty"`
}

// SessionInfo represents session information in responses.
type SessionInfo struct {
	ID             uuid.UUID `json:"id"`
	IPAddress      string    `json:"ip_address"`
	UserAgent      *string   `json:"user_agent,omitempty"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	LastActivityAt time.Time `json:"last_activity_at"`
}

// Login handles POST /auth/login
// Authenticates a member and creates a new session.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate request
	var validationErrors []ErrorDetail
	if req.Email == "" {
		validationErrors = append(validationErrors, NewFieldError("email", "REQUIRED", "Email is required"))
	}
	if req.Password == "" {
		validationErrors = append(validationErrors, NewFieldError("password", "REQUIRED", "Password is required"))
	}
	if len(validationErrors) > 0 {
		WriteError(w, r, NewValidationError("Validation failed", validationErrors))
		return
	}

	// Get client IP
	clientIP := getClientIP(r)

	// Get user agent
	var userAgent *string
	if ua := r.Header.Get("User-Agent"); ua != "" {
		userAgent = &ua
	}

	// Attempt login
	result, err := h.sessionService.Login(r.Context(), auth.LoginRequest{
		Email:     req.Email,
		Password:  req.Password,
		IPAddress: clientIP,
		UserAgent: userAgent,
	})
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("login failed",
				"email", req.Email,
				"ip", clientIP,
				"error", err.Error(),
			)
		}

		// Map domain errors to appropriate responses
		switch {
		case errors.Is(err, domain.ErrInvalidCredentials):
			WriteError(w, r, &ProblemDetails{
				Type:   ErrorTypeUnauthorized,
				Title:  "Invalid Credentials",
				Status: http.StatusUnauthorized,
				Detail: "The email or password is incorrect",
				Code:   CodeUnauthorized,
			})
		case errors.Is(err, domain.ErrMemberNotActivated):
			WriteError(w, r, &ProblemDetails{
				Type:   ErrorTypeUnauthorized,
				Title:  "Account Not Activated",
				Status: http.StatusUnauthorized,
				Detail: "Your account has not been activated. Please check your email for activation instructions.",
				Code:   "ACCOUNT_NOT_ACTIVATED",
			})
		case errors.Is(err, domain.ErrMemberSuspended):
			WriteError(w, r, &ProblemDetails{
				Type:   ErrorTypeMemberSuspended,
				Title:  "Account Suspended",
				Status: http.StatusForbidden,
				Detail: "Your account has been suspended. Please contact support.",
				Code:   CodeForbidden,
			})
		case errors.Is(err, domain.ErrMemberRemoved):
			WriteError(w, r, &ProblemDetails{
				Type:   ErrorTypeForbidden,
				Title:  "Account Removed",
				Status: http.StatusForbidden,
				Detail: "Your account has been removed from the organization.",
				Code:   CodeForbidden,
			})
		default:
			WriteError(w, r, NewInternalError("An error occurred during login"))
		}
		return
	}

	if h.logger != nil {
		h.logger.Info("login successful",
			"member_id", result.Member.ID,
			"organization_id", result.OrganizationID,
			"ip", clientIP,
		)
	}

	// Build response
	response := LoginResponse{
		Token:     result.SessionToken,
		ExpiresAt: result.ExpiresAt,
		SessionID: result.SessionID,
		Member: MemberInfo{
			ID:             result.Member.ID,
			OrganizationID: result.Member.OrganizationID,
			Email:          result.Member.Email,
			Name:           result.Member.Name,
			Status:         string(result.Member.Status),
			JoinedAt:       result.Member.JoinedAt,
			LastLoginAt:    result.Member.LastLoginAt,
		},
		Role: RoleInfo{
			ID:          result.Role.ID,
			Name:        result.Role.Name,
			Permissions: result.Role.Permissions,
			IsSystem:    result.Role.IsSystem,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// Register handles POST /auth/register
// Creates a new organization with an owner member and logs them in.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate request
	var validationErrors []ErrorDetail
	if req.OrganizationName == "" {
		validationErrors = append(validationErrors, NewFieldError("organization_name", "REQUIRED", "Organization name is required"))
	}
	if req.Email == "" {
		validationErrors = append(validationErrors, NewFieldError("email", "REQUIRED", "Email is required"))
	}
	if req.Name == "" {
		validationErrors = append(validationErrors, NewFieldError("name", "REQUIRED", "Name is required"))
	}
	if req.Password == "" {
		validationErrors = append(validationErrors, NewFieldError("password", "REQUIRED", "Password is required"))
	} else if len(req.Password) < 8 {
		validationErrors = append(validationErrors, NewFieldError("password", "MIN_LENGTH", "Password must be at least 8 characters"))
	}
	if len(validationErrors) > 0 {
		WriteError(w, r, NewValidationError("Validation failed", validationErrors))
		return
	}

	// Get client IP
	clientIP := getClientIP(r)

	// Get user agent
	var userAgent *string
	if ua := r.Header.Get("User-Agent"); ua != "" {
		userAgent = &ua
	}

	// Attempt registration
	result, err := h.sessionService.Register(r.Context(), auth.RegisterRequest{
		OrganizationName: req.OrganizationName,
		Email:            req.Email,
		Name:             req.Name,
		Password:         req.Password,
		IPAddress:        clientIP,
		UserAgent:        userAgent,
	})
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("registration failed",
				"email", req.Email,
				"organization_name", req.OrganizationName,
				"ip", clientIP,
				"error", err.Error(),
			)
		}

		// Map domain errors to appropriate responses
		switch {
		case errors.Is(err, domain.ErrMemberEmailTaken):
			WriteError(w, r, &ProblemDetails{
				Type:   ErrorTypeConflict,
				Title:  "Email Already Taken",
				Status: http.StatusConflict,
				Detail: "An account with this email address already exists",
				Code:   "EMAIL_TAKEN",
			})
		case errors.Is(err, domain.ErrOrganizationSlugTaken):
			WriteError(w, r, &ProblemDetails{
				Type:   ErrorTypeConflict,
				Title:  "Organization Name Conflict",
				Status: http.StatusConflict,
				Detail: "An organization with a similar name already exists",
				Code:   "ORGANIZATION_SLUG_TAKEN",
			})
		default:
			WriteError(w, r, NewInternalError("An error occurred during registration"))
		}
		return
	}

	if h.logger != nil {
		h.logger.Info("registration successful",
			"member_id", result.Member.ID,
			"organization_id", result.OrganizationID,
			"ip", clientIP,
		)
	}

	// Build response
	response := RegisterResponse{
		Token:          result.SessionToken,
		ExpiresAt:      result.ExpiresAt,
		SessionID:      result.SessionID,
		OrganizationID: result.OrganizationID,
		Member: MemberInfo{
			ID:             result.Member.ID,
			OrganizationID: result.Member.OrganizationID,
			Email:          result.Member.Email,
			Name:           result.Member.Name,
			Status:         string(result.Member.Status),
			JoinedAt:       result.Member.JoinedAt,
			LastLoginAt:    result.Member.LastLoginAt,
		},
		Role: RoleInfo{
			ID:          result.Role.ID,
			Name:        result.Role.Name,
			Permissions: result.Role.Permissions,
			IsSystem:    result.Role.IsSystem,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

// Logout handles POST /auth/logout
// Invalidates the current session.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Get session token from Authorization header
	sessionToken := extractBearerToken(r)
	if sessionToken == "" {
		WriteError(w, r, NewUnauthorizedError("No session token provided"))
		return
	}

	// Logout
	if err := h.sessionService.Logout(r.Context(), sessionToken); err != nil {
		if h.logger != nil {
			h.logger.Error("logout failed", "error", err.Error())
		}
		// Don't expose internal errors, just return success
	}

	if h.logger != nil {
		h.logger.Info("logout successful")
	}

	w.WriteHeader(http.StatusNoContent)
}

// Me handles GET /auth/me
// Returns the current authenticated member's information.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	// Get session token from Authorization header
	sessionToken := extractBearerToken(r)
	if sessionToken == "" {
		WriteError(w, r, NewUnauthorizedError("No session token provided"))
		return
	}

	// Validate session and get member info
	result, err := h.sessionService.ValidateSession(r.Context(), sessionToken)
	if err != nil {
		if errors.Is(err, domain.ErrSessionNotFound) || errors.Is(err, domain.ErrSessionExpired) {
			WriteError(w, r, &ProblemDetails{
				Type:   ErrorTypeSessionExpired,
				Title:  "Session Invalid",
				Status: http.StatusUnauthorized,
				Detail: "Your session is invalid or has expired. Please log in again.",
				Code:   CodeUnauthorized,
			})
			return
		}
		WriteError(w, r, NewInternalError("Failed to validate session"))
		return
	}

	// Build response
	response := MeResponse{
		Member: MemberInfo{
			ID:             result.Member.ID,
			OrganizationID: result.Member.OrganizationID,
			Email:          result.Member.Email,
			Name:           result.Member.Name,
			Status:         string(result.Member.Status),
			JoinedAt:       result.Member.JoinedAt,
			LastLoginAt:    result.Member.LastLoginAt,
		},
		Role: RoleInfo{
			ID:          result.Role.ID,
			Name:        result.Role.Name,
			Permissions: result.Role.Permissions,
			IsSystem:    result.Role.IsSystem,
		},
		Organization: OrganizationInfo{
			ID: result.OrganizationID,
		},
		Session: SessionInfo{
			ID:             result.Session.ID,
			IPAddress:      result.Session.IPAddress,
			UserAgent:      result.Session.UserAgent,
			ExpiresAt:      result.Session.ExpiresAt,
			CreatedAt:      result.Session.CreatedAt,
			LastActivityAt: result.Session.LastActivityAt,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// LogoutAll handles POST /auth/logout-all
// Invalidates all sessions for the current member.
func (h *AuthHandler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	// Get session token from Authorization header
	sessionToken := extractBearerToken(r)
	if sessionToken == "" {
		WriteError(w, r, NewUnauthorizedError("No session token provided"))
		return
	}

	// Validate session to get member ID
	result, err := h.sessionService.ValidateSession(r.Context(), sessionToken)
	if err != nil {
		WriteError(w, r, NewUnauthorizedError("Invalid session"))
		return
	}

	// Logout all sessions
	if err := h.sessionService.LogoutAll(r.Context(), result.Member.ID); err != nil {
		if h.logger != nil {
			h.logger.Error("logout all failed", "error", err.Error())
		}
		WriteError(w, r, NewInternalError("Failed to logout all sessions"))
		return
	}

	if h.logger != nil {
		h.logger.Info("logout all successful", "member_id", result.Member.ID)
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListSessions handles GET /auth/sessions
// Lists all active sessions for the current member.
func (h *AuthHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	// Get session token from Authorization header
	sessionToken := extractBearerToken(r)
	if sessionToken == "" {
		WriteError(w, r, NewUnauthorizedError("No session token provided"))
		return
	}

	// Validate session to get member ID
	result, err := h.sessionService.ValidateSession(r.Context(), sessionToken)
	if err != nil {
		WriteError(w, r, NewUnauthorizedError("Invalid session"))
		return
	}

	// List sessions
	sessions, err := h.sessionService.ListSessions(r.Context(), result.Member.ID)
	if err != nil {
		if h.logger != nil {
			h.logger.Error("list sessions failed", "error", err.Error())
		}
		WriteError(w, r, NewInternalError("Failed to list sessions"))
		return
	}

	// Build response
	sessionInfos := make([]SessionInfo, len(sessions))
	for i, s := range sessions {
		sessionInfos[i] = SessionInfo{
			ID:             s.ID,
			IPAddress:      s.IPAddress,
			UserAgent:      s.UserAgent,
			ExpiresAt:      s.ExpiresAt,
			CreatedAt:      s.CreatedAt,
			LastActivityAt: s.LastActivityAt,
		}
	}

	response := struct {
		Sessions []SessionInfo `json:"sessions"`
	}{
		Sessions: sessionInfos,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// DeleteSession handles DELETE /auth/sessions/{id}
// Deletes a specific session.
func (h *AuthHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	// Get session ID from URL
	sessionIDStr := chi.URLParam(r, "id")
	if sessionIDStr == "" {
		WriteError(w, r, NewBadRequestError("Session ID is required"))
		return
	}

	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid session ID format"))
		return
	}

	// Get current session token
	sessionToken := extractBearerToken(r)
	if sessionToken == "" {
		WriteError(w, r, NewUnauthorizedError("No session token provided"))
		return
	}

	// Validate current session to get member ID
	result, err := h.sessionService.ValidateSession(r.Context(), sessionToken)
	if err != nil {
		WriteError(w, r, NewUnauthorizedError("Invalid session"))
		return
	}

	// Get the session to delete to verify ownership
	sessions, err := h.sessionService.ListSessions(r.Context(), result.Member.ID)
	if err != nil {
		WriteError(w, r, NewInternalError("Failed to verify session ownership"))
		return
	}

	// Check if the session belongs to the current member
	found := false
	for _, s := range sessions {
		if s.ID == sessionID {
			found = true
			break
		}
	}

	if !found {
		WriteError(w, r, NewNotFoundError("Session"))
		return
	}

	// Delete the session
	if err := h.sessionService.DeleteSession(r.Context(), sessionID); err != nil {
		if h.logger != nil {
			h.logger.Error("delete session failed", "error", err.Error())
		}
		WriteError(w, r, NewInternalError("Failed to delete session"))
		return
	}

	if h.logger != nil {
		h.logger.Info("session deleted", "session_id", sessionID, "member_id", result.Member.ID)
	}

	w.WriteHeader(http.StatusNoContent)
}

// extractBearerToken extracts the bearer token from the Authorization header.
func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}

	return parts[1]
}

// getClientIP extracts the client IP address from the request.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxied requests)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	// RemoteAddr is in the format "IP:port"
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}

	return addr
}
