package rbac

import (
	"net/http"

	"ai-buddy/core/internal/platform/errors"
	"ai-buddy/core/internal/platform/reqctx"
)

func RequireScopes(required ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := reqctx.PrincipalFrom(r.Context())
			if !ok {
				errors.Write(w, r.Context(), http.StatusUnauthorized,
					errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
				return
			}

			if !HasAllScopes(principal.Scopes, required) {
				errors.Write(w, r.Context(), http.StatusForbidden,
					errors.New(errors.CodeForbidden, "Недостаточно прав для этого действия").
						WithDetail("scope", MissingScope(principal.Scopes, required)).
						WithDetail("role", principal.Role))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
