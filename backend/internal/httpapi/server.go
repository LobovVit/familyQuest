package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lobov/familyquest/backend/internal/store"
)

type Server struct {
	store      *store.Store
	corsOrigin string
	mux        *http.ServeMux
}

func NewServer(store *store.Store, corsOrigin string) http.Handler {
	server := &Server{
		store:      store,
		corsOrigin: corsOrigin,
		mux:        http.NewServeMux(),
	}
	server.routes()
	return server
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", s.corsOrigin)
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	s.mux.HandleFunc("POST /api/session", s.verifySession)
	s.mux.HandleFunc("GET /api/participants", s.listParticipants)
	s.mux.HandleFunc("POST /api/participants", s.createParticipant)
	s.mux.HandleFunc("PUT /api/participants/", s.updateParticipantPIN)
	s.mux.HandleFunc("DELETE /api/participants/", s.deleteParticipant)
	s.mux.HandleFunc("GET /api/chores", s.listChores)
	s.mux.HandleFunc("POST /api/chores", s.createChore)
	s.mux.HandleFunc("PUT /api/chores/", s.updateChore)
	s.mux.HandleFunc("GET /api/assignments", s.listAssignments)
	s.mux.HandleFunc("POST /api/assignments", s.createAssignment)
	s.mux.HandleFunc("GET /api/tasks", s.listTasks)
	s.mux.HandleFunc("GET /api/week-plan", s.weekPlan)
	s.mux.HandleFunc("POST /api/tasks/", s.taskAction)
	s.mux.HandleFunc("GET /api/leaderboard", s.leaderboard)
	s.mux.HandleFunc("POST /api/behavior-ratings", s.rateBehavior)
	s.mux.HandleFunc("GET /api/rewards", s.listRewards)
	s.mux.HandleFunc("POST /api/rewards", s.createReward)
	s.mux.HandleFunc("DELETE /api/rewards/", s.deleteReward)
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
	participant, err := s.store.VerifyParticipantPIN(r.Context(), request.ParticipantID, request.PIN)
	respond(w, participant, err)
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
	participant, err := s.store.CreateParticipant(r.Context(), store.Participant{Name: request.Name, Role: request.Role}, request.PIN)
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
	var chore store.Chore
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
	var chore store.Chore
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
		task, err := s.store.CompleteTask(r.Context(), id, request.ParticipantID)
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
		task, err := s.store.ConfirmTask(r.Context(), id, request.ParticipantID, request.Rating, request.Comment)
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
	behavior, err := s.store.RateBehavior(r.Context(), date, request.RaterParticipantID, request.TargetParticipantID, request.Rating, request.Comment)
	respondCreated(w, behavior, err)
}

func (s *Server) listRewards(w http.ResponseWriter, r *http.Request) {
	rewards, err := s.store.ListRewards(r.Context())
	respond(w, rewards, err)
}

func (s *Server) createReward(w http.ResponseWriter, r *http.Request) {
	var reward store.Reward
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
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		if errors.Is(err, store.ErrInvalidRating) {
			status = http.StatusBadRequest
		}
		if errors.Is(err, store.ErrInvalidPIN) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func respondCreated(w http.ResponseWriter, payload any, err error) {
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
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
