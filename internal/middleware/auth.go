package middleware

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/appraisal-crm/request-service/internal/httputil"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type contextKey string

const (
	contextKeyUserID contextKey = "user_id"
	contextKeyRoles  contextKey = "roles"
)

type Claims struct {
	jwt.RegisteredClaims
	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

func Auth(jwks keyfunc.Keyfunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				httputil.RespondError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

			var claims Claims
			_, err := jwt.ParseWithClaims(tokenStr, &claims, jwks.Keyfunc)
			if err != nil {
				httputil.RespondError(w, http.StatusUnauthorized, "invalid token")
				return
			}

			userID, err := uuid.Parse(claims.Subject)
			if err != nil {
				httputil.RespondError(w, http.StatusUnauthorized, "invalid token subject")
				return
			}

			ctx := ContextWithUserID(r.Context(), userID)
			ctx = ContextWithRoles(ctx, claims.RealmAccess.Roles)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ContextWithUserID returns a copy of ctx carrying the authenticated user ID.
func ContextWithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, contextKeyUserID, id)
}

// ContextWithRoles returns a copy of ctx carrying the caller's realm roles.
func ContextWithRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, contextKeyRoles, roles)
}

func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(contextKeyUserID).(uuid.UUID)
	return id, ok
}

func RolesFromContext(ctx context.Context) []string {
	roles, _ := ctx.Value(contextKeyRoles).([]string)
	return roles
}

func HasRole(ctx context.Context, role string) bool {
	return slices.Contains(RolesFromContext(ctx), role)
}

func RequireRoles(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, role := range roles {
				if HasRole(r.Context(), role) {
					next.ServeHTTP(w, r)
					return
				}
			}
			httputil.RespondError(w, http.StatusForbidden, "forbidden")
		})
	}
}
