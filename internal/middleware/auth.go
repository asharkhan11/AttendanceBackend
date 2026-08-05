package middleware

import (
	"net/http"
	"strings"

	"github.com/school/attendance-backend/internal/apperror"
	"github.com/school/attendance-backend/internal/auth"
	"github.com/school/attendance-backend/internal/httpx"
	"github.com/school/attendance-backend/internal/models"
	"github.com/school/attendance-backend/internal/repository"
)

// ─── JWT Auth ─────────────────────────────────────────────────────────────────

func JWTAuth(jwtSvc *auth.JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				httpx.WriteError(w, apperror.Unauthorized("missing or malformed authorization header"))
				return
			}
			token := strings.TrimPrefix(header, "Bearer ")
			claims, err := jwtSvc.VerifyAccessToken(token)
			if err != nil {
				httpx.WriteError(w, apperror.Unauthorized("invalid or expired token"))
				return
			}
			ctx := auth.ContextWithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ─── Tenant Context ───────────────────────────────────────────────────────────
// Opens a DB transaction and sets app.current_school_id so RLS applies.
// The transaction is committed after the handler returns (or rolled back on panic).

func TenantContext(db *repository.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := auth.ClaimsFromContext(r.Context())
			if claims == nil {
				httpx.WriteError(w, apperror.Unauthorized("no claims in context"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ─── RBAC ─────────────────────────────────────────────────────────────────────

// RequireRole enforces that the authenticated user has one of the given roles.
func RequireRole(roles ...models.UserRole) func(http.Handler) http.Handler {
	allowed := make(map[models.UserRole]bool)
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := auth.ClaimsFromContext(r.Context())
			if claims == nil {
				httpx.WriteError(w, apperror.Unauthorized("not authenticated"))
				return
			}
			if !allowed[claims.Role] {
				httpx.WriteError(w, apperror.Forbidden("insufficient role"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
