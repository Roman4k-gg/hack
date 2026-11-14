package api

import (
	"ai-buddy/core/internal/auth"
	"ai-buddy/core/internal/chat"
	"ai-buddy/core/internal/kb"
	"ai-buddy/core/internal/onboarding"
	"ai-buddy/core/internal/team"

	"github.com/go-chi/chi/v5"
)

type Deps struct {
	Auth       *auth.Handler
	Onboarding *onboarding.Handler
	Chat       *chat.Handler
	Team       *team.Handler
	KB         *kb.Handler
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
	})

	return router
}
