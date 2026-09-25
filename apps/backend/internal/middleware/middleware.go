package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

type contextKey string

const userIDKey contextKey = "auth.user_id"

// Authenticator validates a bearer token and puts the caller's id in the request context.
// Handlers read it with UserID, never from the path or body, so ownership cannot be forged.
func Authenticator(tokens *utils.TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				utils.Error(w, http.StatusUnauthorized, "Authorization header is required", nil)
				return
			}

			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
				utils.Error(w, http.StatusUnauthorized, "Authorization header must use the Bearer scheme", nil)
				return
			}

			claims, err := tokens.Parse(strings.TrimSpace(parts[1]))
			if err != nil {
				utils.WriteError(w, err)
				return
			}

			userID, err := claims.UserID()
			if err != nil {
				utils.WriteError(w, err)
				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, userID)))
		})
	}
}

// UserID returns the authenticated caller. The error path matters: a handler mounted
// without the authenticator would otherwise read a zero id and query another user's rows.
func UserID(ctx context.Context) (int64, error) {
	userID, ok := ctx.Value(userIDKey).(int64)
	if !ok || userID == 0 {
		return 0, utils.Unauthorized("Authentication required")
	}
	return userID, nil
}

// SecureHeaders sets the defensive headers that apply to a JSON API.
func SecureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "no-referrer")
		header.Set("Cross-Origin-Opener-Policy", "same-origin")
		header.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		next.ServeHTTP(w, r)
	})
}

// RequestLogger emits one structured line per request. It never logs the Authorization
// header or a request body, so credentials stay out of the log.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(wrapped, r)

		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.Status(),
			"bytes", wrapped.BytesWritten(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", middleware.GetReqID(r.Context()),
			"remote_ip", r.RemoteAddr,
		)
	})
}
