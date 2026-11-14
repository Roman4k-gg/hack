package api

import (
	"ai-buddy/core/internal/analytics"
	"ai-buddy/core/internal/auth"
	"ai-buddy/core/internal/chat"
	"ai-buddy/core/internal/flags"
	"ai-buddy/core/internal/kb"
	"ai-buddy/core/internal/onboarding"
	"ai-buddy/core/internal/requests"
	"ai-buddy/core/internal/team"

	"github.com/go-chi/chi/v5"
)

type Deps struct {
	Auth       *auth.Handler
	Onboarding *onboarding.Handler
	Chat       *chat.Handler
	Team       *team.Handler
	KB         *kb.Handler
	Requests   *requests.Handler
	Flags      *flags.Handler
	Analytics  *analytics.Handler
	Tokens     *auth.TokenManager
}

func New(deps Deps) chi.Router {
	router := chi.NewRouter()

	router.Post("/auth/login", deps.Auth.Login())

	router.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth(deps.Tokens))

		r.Get("/me", deps.Auth.Me())
		r.Mount("/onboarding", deps.Onboarding.Router())
		r.Mount("/chat", deps.Chat.Router())
		r.Mount("/team", deps.Team.Router())
		r.Mount("/kb", deps.KB.Router())
		r.Mount("/requests", deps.Requests.Router())
		r.Mount("/analytics", deps.Analytics.Router())

		admin := chi.NewRouter()
		admin.Mount("/feature-flags", deps.Flags.Router())
		admin.Mount("/analytics", deps.Analytics.AdminRouter())

		r.Mount("/admin", admin)
	})

	return router
}
