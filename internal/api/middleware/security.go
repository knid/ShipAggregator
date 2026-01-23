// Package middleware provides HTTP middleware for the shipping aggregator API.
package middleware

import (
	"net/http"
	"strings"
)

// SecurityHeadersConfig holds configuration for security headers middleware.
type SecurityHeadersConfig struct {
	// ContentSecurityPolicy sets the Content-Security-Policy header.
	ContentSecurityPolicy string
	// XContentTypeOptions sets the X-Content-Type-Options header.
	XContentTypeOptions string
	// XFrameOptions sets the X-Frame-Options header.
	XFrameOptions string
	// XXSSProtection sets the X-XSS-Protection header.
	XXSSProtection string
	// StrictTransportSecurity sets the Strict-Transport-Security header.
	StrictTransportSecurity string
	// ReferrerPolicy sets the Referrer-Policy header.
	ReferrerPolicy string
	// PermissionsPolicy sets the Permissions-Policy header.
	PermissionsPolicy string
	// CacheControl sets the Cache-Control header for API responses.
	CacheControl string
	// EnableHSTS enables HSTS header (should be true in production).
	EnableHSTS bool
}

// DefaultSecurityHeadersConfig returns a default security headers configuration.
func DefaultSecurityHeadersConfig() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		ContentSecurityPolicy:   "default-src 'none'; frame-ancestors 'none'",
		XContentTypeOptions:     "nosniff",
		XFrameOptions:           "DENY",
		XXSSProtection:          "1; mode=block",
		StrictTransportSecurity: "max-age=31536000; includeSubDomains",
		ReferrerPolicy:          "strict-origin-when-cross-origin",
		PermissionsPolicy:       "geolocation=(), microphone=(), camera=()",
		CacheControl:            "no-store, no-cache, must-revalidate, proxy-revalidate",
		EnableHSTS:              false, // Enable in production
	}
}

// SecurityHeaders returns a middleware that adds security headers to responses.
func SecurityHeaders(cfg SecurityHeadersConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set security headers
			if cfg.ContentSecurityPolicy != "" {
				w.Header().Set("Content-Security-Policy", cfg.ContentSecurityPolicy)
			}
			if cfg.XContentTypeOptions != "" {
				w.Header().Set("X-Content-Type-Options", cfg.XContentTypeOptions)
			}
			if cfg.XFrameOptions != "" {
				w.Header().Set("X-Frame-Options", cfg.XFrameOptions)
			}
			if cfg.XXSSProtection != "" {
				w.Header().Set("X-XSS-Protection", cfg.XXSSProtection)
			}
			if cfg.EnableHSTS && cfg.StrictTransportSecurity != "" {
				w.Header().Set("Strict-Transport-Security", cfg.StrictTransportSecurity)
			}
			if cfg.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", cfg.ReferrerPolicy)
			}
			if cfg.PermissionsPolicy != "" {
				w.Header().Set("Permissions-Policy", cfg.PermissionsPolicy)
			}
			if cfg.CacheControl != "" {
				w.Header().Set("Cache-Control", cfg.CacheControl)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SecureHeaders returns a middleware with default security headers.
func SecureHeaders() func(http.Handler) http.Handler {
	return SecurityHeaders(DefaultSecurityHeadersConfig())
}

// CORSConfig holds configuration for CORS middleware.
type CORSConfig struct {
	// AllowedOrigins is a list of allowed origins.
	AllowedOrigins []string
	// AllowedMethods is a list of allowed HTTP methods.
	AllowedMethods []string
	// AllowedHeaders is a list of allowed headers.
	AllowedHeaders []string
	// ExposedHeaders is a list of headers exposed to the client.
	ExposedHeaders []string
	// AllowCredentials indicates if credentials are allowed.
	AllowCredentials bool
	// MaxAge is the max age for preflight cache in seconds.
	MaxAge int
}

// DefaultCORSConfig returns a default CORS configuration.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"Idempotency-Key",
			"X-Request-ID",
		},
		ExposedHeaders: []string{
			"X-Request-ID",
			"X-RateLimit-Limit",
			"X-RateLimit-Remaining",
			"X-RateLimit-Reset",
			"Retry-After",
		},
		AllowCredentials: false,
		MaxAge:           86400, // 24 hours
	}
}

// CORS returns a middleware that handles CORS.
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			if origin != "" && isOriginAllowed(origin, cfg.AllowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)

				if cfg.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}

				// Handle preflight request
				if r.Method == http.MethodOptions {
					w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
					w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
					if len(cfg.ExposedHeaders) > 0 {
						w.Header().Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposedHeaders, ", "))
					}
					if cfg.MaxAge > 0 {
						w.Header().Set("Access-Control-Max-Age", string(rune(cfg.MaxAge)))
					}
					w.WriteHeader(http.StatusNoContent)
					return
				}

				// Set exposed headers for actual requests
				if len(cfg.ExposedHeaders) > 0 {
					w.Header().Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposedHeaders, ", "))
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isOriginAllowed checks if an origin is in the allowed list.
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	for _, allowed := range allowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
		// Support wildcard subdomains like *.example.com
		if strings.HasPrefix(allowed, "*.") {
			domain := strings.TrimPrefix(allowed, "*")
			if strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}
	return false
}

// RequestSizeLimit returns a middleware that limits request body size.
func RequestSizeLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// ContentTypeValidator returns a middleware that validates Content-Type header.
func ContentTypeValidator(allowedTypes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip validation for methods that don't have a body
			if r.Method == http.MethodGet || r.Method == http.MethodHead ||
				r.Method == http.MethodDelete || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			// Skip if no body
			if r.ContentLength == 0 {
				next.ServeHTTP(w, r)
				return
			}

			contentType := r.Header.Get("Content-Type")
			if contentType == "" {
				http.Error(w, "Content-Type header required", http.StatusUnsupportedMediaType)
				return
			}

			// Extract media type (ignore parameters like charset)
			mediaType := strings.Split(contentType, ";")[0]
			mediaType = strings.TrimSpace(mediaType)

			for _, allowed := range allowedTypes {
				if strings.EqualFold(mediaType, allowed) {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, "Unsupported Content-Type", http.StatusUnsupportedMediaType)
		})
	}
}

// JSONContentType returns a middleware that validates JSON Content-Type.
func JSONContentType() func(http.Handler) http.Handler {
	return ContentTypeValidator("application/json")
}

// NoCache returns a middleware that sets no-cache headers.
func NoCache() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			next.ServeHTTP(w, r)
		})
	}
}

// SanitizeHeaders returns a middleware that removes potentially dangerous headers.
func SanitizeHeaders() func(http.Handler) http.Handler {
	// Headers that should be removed from incoming requests
	dangerousHeaders := []string{
		"X-Forwarded-Host",
		"X-Original-URL",
		"X-Rewrite-URL",
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, header := range dangerousHeaders {
				r.Header.Del(header)
			}
			next.ServeHTTP(w, r)
		})
	}
}
