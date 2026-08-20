package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lobov/familyquest/backend/internal/application"
	"github.com/lobov/familyquest/backend/internal/domain"
)

type Server struct {
	store      *application.Service
	corsOrigin string
	mux        *http.ServeMux
}

func NewServer(service *application.Service, corsOrigin string) http.Handler {
	server := &Server{
		store:      service,
		corsOrigin: corsOrigin,
		mux:        http.NewServeMux(),
	}
	server.routes()
	return server
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", s.corsOrigin)
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		if err := s.store.Ready(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	s.mux.HandleFunc("POST /api/session", s.verifySession)
	s.mux.HandleFunc("GET /api/participants", s.listParticipants)
	s.mux.Handle("POST /api/participants", s.authorize(true, s.createParticipant))
	s.mux.Handle("PUT /api/participants/", s.authorize(true, s.updateParticipantPIN))
	s.mux.Handle("DELETE /api/participants/", s.authorize(true, s.deleteParticipant))
	s.mux.Handle("GET /api/chores", s.authorize(false, s.listChores))
	s.mux.Handle("POST /api/chores", s.authorize(true, s.createChore))
	s.mux.Handle("PUT /api/chores/", s.authorize(true, s.updateChore))
	s.mux.Handle("GET /api/assignments", s.authorize(false, s.listAssignments))
	s.mux.Handle("POST /api/assignments", s.authorize(true, s.createAssignment))
	s.mux.Handle("GET /api/tasks", s.authorize(false, s.listTasks))
	s.mux.Handle("GET /api/week-plan", s.authorize(false, s.weekPlan))
	s.mux.Handle("POST /api/tasks/", s.authorize(false, s.taskAction))
	s.mux.Handle("GET /api/leaderboard", s.authorize(false, s.leaderboard))
	s.mux.Handle("GET /api/behavior-ratings", s.authorize(false, s.listBehaviorRatings))
	s.mux.Handle("POST /api/behavior-ratings", s.authorize(true, s.rateBehavior))
	s.mux.Handle("GET /api/rewards", s.authorize(false, s.listRewards))
	s.mux.Handle("POST /api/rewards", s.authorize(true, s.createReward))
	s.mux.Handle("DELETE /api/rewards/", s.authorize(true, s.deleteReward))
	s.mux.Handle("GET /api/backup", s.authorize(true, s.exportBackup))
	s.mux.Handle("POST /api/backup", s.authorize(true, s.importBackup))
}

type principalKey struct{}

func (s *Server) authorize(parentOnly bool, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := s.store.ParseToken(r.Header.Get("Authorization"))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if parentOnly && !p.IsParent() {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	})
}
func principal(r *http.Request) domain.Principal {
	p, _ := r.Context().Value(principalKey{}).(domain.Principal)
	return p
}

func (s *Server) verifySession(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ParticipantID int64  `json:"participantId"`
		PIN           string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if len(request.PIN) != 6 {
		writeError(w, http.StatusBadRequest, "pin must contain 6 digits")
		return
	}
	participant, token, err := s.store.Authenticate(r.Context(), request.ParticipantID, request.PIN)
	respond(w, map[string]any{"participant": participant, "token": token}, err)
}

func (s *Server) listParticipants(w http.ResponseWriter, r *http.Request) {
	participants, err := s.store.ListParticipants(r.Context())
	respond(w, participants, err)
}

func (s *Server) createParticipant(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name string `json:"name"`
		Role string `json:"role"`
		PIN  string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if request.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(request.PIN) != 6 {
		writeError(w, http.StatusBadRequest, "pin must contain 6 digits")
		return
	}
	participant, err := s.store.CreateParticipant(r.Context(), domain.Participant{Name: request.Name, Role: request.Role}, request.PIN)
	respondCreated(w, participant, err)
}

func (s *Server) deleteParticipant(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(r.URL.Path, "api", "participants")
	if !ok {
		writeError(w, http.StatusNotFound, "unknown participant")
		return
	}
	err := s.store.DeleteParticipant(r.Context(), id)
	respond(w, map[string]string{"status": "deleted"}, err)
}

func (s *Server) updateParticipantPIN(w http.ResponseWriter, r *http.Request) {
	id, ok := parseActionPath(r.URL.Path, "api", "participants", "pin")
	if !ok {
		writeError(w, http.StatusNotFound, "unknown participant action")
		return
	}
	var request struct {
		PIN string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if len(request.PIN) != 6 {
		writeError(w, http.StatusBadRequest, "pin must contain 6 digits")
		return
	}
	participant, err := s.store.UpdateParticipantPIN(r.Context(), id, request.PIN)
	respond(w, participant, err)
}

func (s *Server) listChores(w http.ResponseWriter, r *http.Request) {
	chores, err := s.store.ListChores(r.Context())
	respond(w, chores, err)
}

func (s *Server) createChore(w http.ResponseWriter, r *http.Request) {
	var chore domain.Chore
	if err := json.NewDecoder(r.Body).Decode(&chore); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	created, err := s.store.CreateChore(r.Context(), chore)
	respondCreated(w, created, err)
}

func (s *Server) updateChore(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(r.URL.Path, "api", "chores")
	if !ok {
		writeError(w, http.StatusNotFound, "unknown chore")
		return
	}
	var chore domain.Chore
	if err := json.NewDecoder(r.Body).Decode(&chore); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	chore.ID = id
	updated, err := s.store.UpdateChore(r.Context(), chore)
	respond(w, updated, err)
}

func (s *Server) listAssignments(w http.ResponseWriter, r *http.Request) {
	assignments, err := s.store.ListAssignments(r.Context())
	respond(w, assignments, err)
}

func (s *Server) createAssignment(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ChoreID       int64 `json:"choreId"`
		ParticipantID int64 `json:"participantId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	assignment, err := s.store.CreateAssignment(r.Context(), request.ChoreID, request.ParticipantID)
	respondCreated(w, assignment, err)
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	date := parseDate(r.URL.Query().Get("date"))
	tasks, err := s.store.ListTasks(r.Context(), date)
	respond(w, tasks, err)
}

func (s *Server) weekPlan(w http.ResponseWriter, r *http.Request) {
	date := parseDate(r.URL.Query().Get("date"))
	items, err := s.store.ListWeekPlan(r.Context(), date)
	respond(w, items, err)
}

func (s *Server) taskAction(w http.ResponseWriter, r *http.Request) {
	id, action, ok := parseTaskAction(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown task action")
		return
	}

	switch action {
	case "complete":
		var request struct {
			ParticipantID int64 `json:"participantId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		task, err := s.store.CompleteTask(r.Context(), principal(r), id)
		respond(w, task, err)
	case "confirm":
		var request struct {
			ParticipantID int64  `json:"participantId"`
			Rating        int    `json:"rating"`
			Comment       string `json:"comment"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		task, err := s.store.ConfirmTask(r.Context(), principal(r), id, request.Rating, request.Comment)
		respond(w, task, err)
	default:
		writeError(w, http.StatusNotFound, "unknown task action")
	}
}

func (s *Server) leaderboard(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period != "day" && period != "month" {
		period = "week"
	}
	date := parseDate(r.URL.Query().Get("date"))
	entries, err := s.store.Leaderboard(r.Context(), period, date)
	respond(w, entries, err)
}

func (s *Server) rateBehavior(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Date                string `json:"date"`
		RaterParticipantID  int64  `json:"raterParticipantId"`
		TargetParticipantID int64  `json:"targetParticipantId"`
		Rating              int    `json:"rating"`
		Comment             string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	date := parseDate(request.Date)
	behavior, err := s.store.RateBehavior(r.Context(), principal(r), date, request.TargetParticipantID, request.Rating, request.Comment)
	respondCreated(w, behavior, err)
}

func (s *Server) listBehaviorRatings(w http.ResponseWriter, r *http.Request) {
	date := parseDate(r.URL.Query().Get("date"))
	ratings, err := s.store.ListBehaviorRatings(r.Context(), date)
	respond(w, ratings, err)
}

func (s *Server) listRewards(w http.ResponseWriter, r *http.Request) {
	rewards, err := s.store.ListRewards(r.Context())
	respond(w, rewards, err)
}

func (s *Server) createReward(w http.ResponseWriter, r *http.Request) {
	var reward domain.Reward
	if err := json.NewDecoder(r.Body).Decode(&reward); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if reward.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	created, err := s.store.CreateReward(r.Context(), reward)
	respondCreated(w, created, err)
}

func (s *Server) deleteReward(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(r.URL.Path, "api", "rewards")
	if !ok {
		writeError(w, http.StatusNotFound, "unknown reward")
		return
	}
	err := s.store.DeleteReward(r.Context(), id)
	respond(w, map[string]string{"status": "deleted"}, err)
}

func (s *Server) exportBackup(w http.ResponseWriter, r *http.Request) {
	backup, err := s.store.ExportBackup(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="familyquest-backup.json"`)
	writeJSON(w, http.StatusOK, backup)
}

func (s *Server) importBackup(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 20<<20)
	defer r.Body.Close()

	payload, err := io.ReadAll(r.Body)
	if err != nil || len(payload) == 0 {
		writeError(w, http.StatusBadRequest, "invalid backup JSON")
		return
	}
	if err := s.store.ImportBackup(r.Context(), payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	respond(w, map[string]string{"status": "imported"}, nil)
}

func parseTaskAction(path string) (int64, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "tasks" {
		return 0, "", false
	}
	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return 0, "", false
	}
	return id, parts[3], true
}

func parseIDPath(path string, first string, second string) (int64, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 3 || parts[0] != first || parts[1] != second {
		return 0, false
	}
	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

func parseActionPath(path string, first string, second string, action string) (int64, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != first || parts[1] != second || parts[3] != action {
		return 0, false
	}
	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

func parseDate(value string) time.Time {
	if value == "" {
		return time.Now()
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Now()
	}
	return parsed
}

func respond(w http.ResponseWriter, payload any, err error) {
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, domain.ErrNotFound) {
			status = http.StatusNotFound
		}
		if errors.Is(err, domain.ErrInvalidRating) {
			status = http.StatusBadRequest
		}
		if errors.Is(err, domain.ErrInvalidPIN) {
			status = http.StatusUnauthorized
		}
		if errors.Is(err, domain.ErrInvalidPINFormat) || errors.Is(err, domain.ErrInvalidRole) {
			status = http.StatusBadRequest
		}
		if errors.Is(err, domain.ErrForbidden) {
			status = http.StatusForbidden
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func respondCreated(w http.ResponseWriter, payload any, err error) {
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, domain.ErrNotFound) {
			status = http.StatusNotFound
		}
		if errors.Is(err, domain.ErrInvalidPINFormat) || errors.Is(err, domain.ErrInvalidRole) || errors.Is(err, domain.ErrInvalidRating) {
			status = http.StatusBadRequest
		}
		if errors.Is(err, domain.ErrForbidden) {
			status = http.StatusForbidden
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, payload)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
