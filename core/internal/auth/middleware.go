package auth

import (
	"net/http"
	"strings"

	"ai-buddy/core/internal/platform/errors"
	"ai-buddy/core/internal/platform/reqctx"
)

type userIDSetter interface {
	SetUserID(id string)
}

func RequireAuth(tokens *TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := bearerToken(r)
			if !ok {
				errors.Write(w, r.Context(), http.StatusUnauthorized,
					errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
				return
			}

			claims, err := tokens.Parse(raw)
			if err != nil {
				errors.Write(w, r.Context(), http.StatusUnauthorized,
					errors.New(errors.CodeUnauthorized, "Токен недействителен или истёк"))
				return
			}

			if setter, ok := w.(userIDSetter); ok {
				setter.SetUserID(claims.Subject)
			}

			ctx := reqctx.WithUserID(r.Context(), claims.Subject)
			ctx = reqctx.WithPrincipal(ctx, reqctx.Principal{
				UserID: claims.Subject,
				Email:  claims.Email,
				Role:   claims.Role,
				TeamID: claims.TeamID,
				Scopes: claims.Scopes,
			})

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return "", false
	}

	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", false
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", false
	}
	return token, true
}
