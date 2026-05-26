package httpapi

import (
	"github.com/cobanov/terminal-army-go/internal/auth"
	"github.com/cobanov/terminal-army-go/internal/svc"
	"github.com/cobanov/terminal-army-go/internal/web"
	"github.com/cobanov/terminal-army-go/internal/ws"
	"github.com/go-chi/chi/v5"
)

func init() {
	mount = func(r chi.Router, app *svc.App, hub *ws.Hub) {
		// Per-IP throttle for credential endpoints: ~10 attempts burst, then
		// 1 per 5 seconds sustained. Generous enough for forgotten passwords
		// and slow typists, tight enough to make credential stuffing painful.
		authThrottle := newRateLimiter(10, 0.2)

		// Public auth endpoints
		r.Route("/api/v1/auth", func(r chi.Router) {
			r.Use(authThrottle.middleware)
			r.Post("/register", authRegister(app))
			r.Post("/login", authLogin(app))
			r.Post("/logout", authLogout(app))
		})

		// Authenticated API
		r.Route("/api/v1", func(r chi.Router) {
			r.Use(auth.Middleware(app))

			r.Get("/me", meHandler(app))
			r.Get("/universes", listUniverses(app))
			r.Post("/universes/{id}/join", joinUniverse(app))

			r.Route("/planets", func(r chi.Router) {
				r.Get("/", listPlanets(app))
				r.Get("/{planetID}", getPlanet(app))
				r.Get("/{planetID}/production", getProduction(app))
				r.Post("/{planetID}/buildings", queueBuilding(app))
				r.Post("/{planetID}/shipyard", queueShip(app))
				r.Post("/{planetID}/defense", queueDefense(app))
				r.Get("/{planetID}/queues", getQueues(app))
			})

			r.Route("/research", func(r chi.Router) {
				r.Get("/", listResearch(app))
				r.Post("/", queueResearch(app))
			})

			r.Route("/galaxy", func(r chi.Router) {
				r.Get("/{galaxy}/{system}", viewSystem(app))
			})

			r.Route("/fleet", func(r chi.Router) {
				r.Post("/", dispatchFleet(app))
				r.Get("/", listFleet(app))
				r.Post("/{fleetID}/recall", recallFleet(app))
			})

			r.Route("/messages", func(r chi.Router) {
				r.Get("/", listMessages(app))
				r.Get("/{id}", getMessage(app))
				r.Delete("/{id}", deleteMessage(app))
			})

			r.Route("/reports", func(r chi.Router) {
				r.Get("/", listReports(app))
				r.Get("/{id}", getReport(app))
			})

			r.Route("/alliance", func(r chi.Router) {
				r.Get("/", listAlliances(app))
				r.Post("/", createAlliance(app))
				r.Get("/{id}", getAlliance(app))
				r.Post("/{id}/join", joinAlliance(app))
				r.Post("/{id}/leave", leaveAlliance(app))
			})

			r.Get("/leaderboard", leaderboardHandler(app))
			r.Get("/stats", statsHandler(app))

			r.Get("/ws", ws.Handler(hub, app))
		})

		web.Mount(r, app)
	}
}
