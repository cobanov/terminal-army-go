package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/cobanov/terminal-army-go/internal/auth"
	"github.com/cobanov/terminal-army-go/internal/svc"
	"github.com/go-chi/chi/v5"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func authRegister(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Username string `json:"username"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		out, err := app.Auth.Register(r.Context(), in.Username, in.Email, in.Password)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, out)
	}
}

func authLogin(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		out, err := app.Auth.Login(r.Context(), in.Username, in.Password)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func authLogout(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := auth.TokenFromRequest(r)
		if tok != "" {
			_ = app.Auth.Logout(r.Context(), tok)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func deviceStart(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := app.Auth.StartDeviceAuth(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func devicePoll(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			AuthCode string `json:"auth_code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		out, err := app.Auth.PollDeviceAuth(r.Context(), in.AuthCode)
		if err != nil {
			switch {
			case errors.Is(err, svc.ErrDevicePending):
				writeError(w, http.StatusAccepted, "pending")
			case errors.Is(err, svc.ErrSessionExpired):
				writeError(w, http.StatusGone, "auth code expired")
			default:
				writeError(w, http.StatusNotFound, "auth code not found")
			}
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func meHandler(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := auth.UserFromContext(r.Context())
		writeJSON(w, http.StatusOK, user)
	}
}

func listUniverses(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		us, err := app.Universe.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, us)
	}
}

func joinUniverse(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid universe id")
			return
		}
		planet, err := app.Universe.JoinUniverse(r.Context(), uid, id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, planet)
	}
}

func listPlanets(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		ps, err := app.Planet.ListByUser(r.Context(), uid)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ps)
	}
}

func planetID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "planetID"), 10, 64)
}

func getPlanet(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		pid, err := planetID(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid planet id")
			return
		}
		p, err := app.Planet.GetForUser(r.Context(), uid, pid)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

func getProduction(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		pid, err := planetID(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid planet id")
			return
		}
		rep, err := app.Planet.Production(r.Context(), uid, pid)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, rep)
	}
}

func queueBuilding(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		pid, err := planetID(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid planet id")
			return
		}
		var in struct {
			Building string `json:"building"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		q, err := app.Build.QueueBuilding(r.Context(), uid, pid, in.Building)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, q)
	}
}

func queueShip(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		pid, err := planetID(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid planet id")
			return
		}
		var in struct {
			Ship  string `json:"ship"`
			Count int    `json:"count"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		q, err := app.Shipyard.QueueShip(r.Context(), uid, pid, in.Ship, in.Count)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, q)
	}
}

func queueDefense(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		pid, err := planetID(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid planet id")
			return
		}
		var in struct {
			Defense string `json:"defense"`
			Count   int    `json:"count"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		q, err := app.Shipyard.QueueDefense(r.Context(), uid, pid, in.Defense, in.Count)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, q)
	}
}

func getQueues(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		pid, err := planetID(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid planet id")
			return
		}
		qs, err := app.Build.PlanetQueues(r.Context(), uid, pid)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, qs)
	}
}

func listResearch(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		rs, err := app.Research.List(r.Context(), uid)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, rs)
	}
}

func queueResearch(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		var in struct {
			Tech     string `json:"tech"`
			PlanetID int64  `json:"planet_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		q, err := app.Research.Queue(r.Context(), uid, in.PlanetID, in.Tech)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, q)
	}
}

func viewSystem(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		g, _ := strconv.Atoi(chi.URLParam(r, "galaxy"))
		s, _ := strconv.Atoi(chi.URLParam(r, "system"))
		view, err := app.Galaxy.ViewSystem(r.Context(), uid, g, s)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, view)
	}
}

func dispatchFleet(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		var in svc.FleetDispatchRequest
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		f, err := app.Fleet.Dispatch(r.Context(), uid, in)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, f)
	}
}

func listFleet(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		fs, err := app.Fleet.List(r.Context(), uid)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, fs)
	}
}

func recallFleet(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		id, _ := strconv.ParseInt(chi.URLParam(r, "fleetID"), 10, 64)
		f, err := app.Fleet.Recall(r.Context(), uid, id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, f)
	}
}

func listMessages(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		ms, err := app.Messages.List(r.Context(), uid)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ms)
	}
}

func sendMessage(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		var in struct {
			To   string `json:"to"`
			Body string `json:"body"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		m, err := app.Messages.Send(r.Context(), uid, in.To, in.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, m)
	}
}

func getMessage(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		m, err := app.Messages.Get(r.Context(), uid, id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, m)
	}
}

func deleteMessage(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err := app.Messages.Delete(r.Context(), uid, id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func listReports(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		rs, err := app.Reports.List(r.Context(), uid)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, rs)
	}
}

func getReport(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		rep, err := app.Reports.Get(r.Context(), uid, id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, rep)
	}
}

func listAlliances(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		as, err := app.Alliance.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, as)
	}
}

func createAlliance(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		var in struct {
			Tag         string `json:"tag"`
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		a, err := app.Alliance.Create(r.Context(), uid, in.Tag, in.Name, in.Description)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, a)
	}
}

func getAlliance(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		a, err := app.Alliance.Get(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, a)
	}
}

func joinAlliance(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err := app.Alliance.Join(r.Context(), uid, id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func leaveAlliance(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := auth.UserIDFromContext(r.Context())
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err := app.Alliance.Leave(r.Context(), uid, id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func leaderboardHandler(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lb, err := app.Leaderboard.Top(r.Context(), 100)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, lb)
	}
}

func statsHandler(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := app.Stats.Overview(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, s)
	}
}

func publicStatsHandler(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := app.Stats.PublicOverview(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, s)
	}
}
