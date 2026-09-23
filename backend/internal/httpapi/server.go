package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log"
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
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Access-Control-Allow-Origin", s.corsOrigin)
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-FamilyQuest, X-FamilyQuest-Confirmation")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	limit := int64(1 << 20)
	if strings.HasPrefix(r.URL.Path, "/api/family") {
		limit = 2 << 20
	}
	if r.URL.Path == "/api/backup" {
		limit = 64 << 20
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.deviceRoutes()
	s.familyRoutes()
	s.learningRoutes()
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
	s.mux.Handle("PUT /api/participants/", s.authorize(true, s.confirmed(s.updateParticipantPIN)))
	s.mux.Handle("DELETE /api/participants/", s.authorize(true, s.deleteParticipant))
	s.mux.Handle("GET /api/chores", s.authorize(false, s.listChores))
	s.mux.Handle("POST /api/chores", s.authorize(true, s.createChore))
	s.mux.Handle("PUT /api/chores/", s.authorize(true, s.updateChore))
	s.mux.Handle("GET /api/assignments", s.authorize(false, s.listAssignments))
	s.mux.Handle("POST /api/assignments", s.authorize(true, s.createAssignment))
	s.mux.Handle("GET /api/tasks", s.authorize(false, s.listTasks))
	s.mux.Handle("GET /api/week-plan", s.authorize(false, s.weekPlan))
	s.mux.Handle("POST /api/tasks/", s.authorize(false, s.taskAction))
	s.mux.HandleFunc("GET /api/leaderboard", s.leaderboard)
	s.mux.Handle("GET /api/behavior-ratings", s.authorize(false, s.listBehaviorRatings))
	s.mux.Handle("POST /api/behavior-ratings", s.authorize(false, s.rateBehavior))
	s.mux.Handle("GET /api/rewards", s.authorize(false, s.listRewards))
	s.mux.Handle("POST /api/rewards", s.authorize(true, s.createReward))
	s.mux.Handle("DELETE /api/rewards/", s.authorize(true, s.deleteReward))
	s.mux.Handle("GET /api/backup", s.authorize(true, s.exportBackup))
	s.mux.Handle("POST /api/backup", s.authorize(true, s.confirmed(s.importBackup)))
}

type principalKey struct{}

func (s *Server) authorize(parentOnly bool, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := s.requestPrincipal(w, r)
		if err != nil {
			respond(w, nil, err)
			return
		}
		if parentOnly && !p.IsParent() {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		next(w, withPrincipal(r, p))
	})
}
func principal(r *http.Request) domain.Principal {
	p, _ := r.Context().Value(principalKey{}).(domain.Principal)
	return p
}

func (s *Server) verifySession(w http.ResponseWriter, r *http.Request) {
	if !sameSiteRequest(r) {
		respond(w, nil, domain.ErrForbidden)
		return
	}
	var request struct {
		Remember      bool   `json:"remember"`
		DeviceName    string `json:"deviceName"`
		ParentID      int64  `json:"parentId"`
		ParentPIN     string `json:"parentPin"`
		ParticipantID int64  `json:"participantId"`
		PIN           string `json:"pin"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if len(request.PIN) != 6 {
		writeError(w, http.StatusBadRequest, "pin must contain 6 digits")
		return
	}
	participant, token, err := s.store.Authenticate(r.Context(), request.ParticipantID, request.PIN)
	if err != nil {
		respond(w, nil, err)
		return
	}
	var secret, deviceID string
	if request.Remember {
		var d domain.TrustedDevice
		secret, d, err = s.store.RememberDevice(r.Context(), participant, request.DeviceName, request.ParentID, request.ParentPIN)
		if err != nil {
			respond(w, nil, err)
			return
		}
		deviceID = d.ID
		token = ""
	}
	if err = s.store.ForgetDevice(r.Context(), cookieValue(r)); err != nil {
		respond(w, nil, err)
		return
	}
	setDeviceCookie(w, r, secret)
	respond(w, map[string]any{"participant": participant, "token": token, "remembered": request.Remember, "deviceId": deviceID}, nil)
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
	if err := decodeJSON(r, &request); err != nil {
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
	participant, err := s.store.CreateParticipant(r.Context(), principal(r), domain.Participant{Name: request.Name, Role: request.Role}, request.PIN)
	respondCreated(w, participant, err)
}

func (s *Server) deleteParticipant(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(r.URL.Path, "api", "participants")
	if !ok {
		writeError(w, http.StatusNotFound, "unknown participant")
		return
	}
	err := s.store.DeleteParticipant(r.Context(), principal(r), id)
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
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if len(request.PIN) != 6 {
		writeError(w, http.StatusBadRequest, "pin must contain 6 digits")
		return
	}
	participant, err := s.store.UpdateParticipantPIN(r.Context(), principal(r), id, request.PIN)
	respond(w, participant, err)
}

func (s *Server) listChores(w http.ResponseWriter, r *http.Request) {
	chores, err := s.store.ListChores(r.Context())
	respond(w, chores, err)
}

func (s *Server) createChore(w http.ResponseWriter, r *http.Request) {
	var chore domain.Chore
	if err := decodeJSON(r, &chore); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	created, err := s.store.CreateChore(r.Context(), principal(r), chore)
	respondCreated(w, created, err)
}

func (s *Server) updateChore(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(r.URL.Path, "api", "chores")
	if !ok {
		writeError(w, http.StatusNotFound, "unknown chore")
		return
	}
	var chore domain.Chore
	if err := decodeJSON(r, &chore); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	chore.ID = id
	updated, err := s.store.UpdateChore(r.Context(), principal(r), chore)
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
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	assignment, err := s.store.CreateAssignment(r.Context(), principal(r), request.ChoreID, request.ParticipantID)
	respondCreated(w, assignment, err)
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	date, err := parseDate(r.URL.Query().Get("date"))
	if err != nil {
		respond(w, nil, err)
		return
	}
	tasks, err := s.store.ListTasks(r.Context(), date)
	respond(w, tasks, err)
}

func (s *Server) weekPlan(w http.ResponseWriter, r *http.Request) {
	date, err := parseDate(r.URL.Query().Get("date"))
	if err != nil {
		respond(w, nil, err)
		return
	}
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
		if err := decodeJSON(r, &request); err != nil {
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
		if err := decodeJSON(r, &request); err != nil {
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
	date, err := parseDate(r.URL.Query().Get("date"))
	if err != nil {
		respond(w, nil, err)
		return
	}
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
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	date, err := parseDate(request.Date)
	if err != nil {
		respond(w, nil, err)
		return
	}
	behavior, err := s.store.RateBehavior(r.Context(), principal(r), date, request.TargetParticipantID, request.Rating, request.Comment)
	respondCreated(w, behavior, err)
}

func (s *Server) listBehaviorRatings(w http.ResponseWriter, r *http.Request) {
	date, err := parseDate(r.URL.Query().Get("date"))
	if err != nil {
		respond(w, nil, err)
		return
	}
	ratings, err := s.store.ListBehaviorRatings(r.Context(), date)
	respond(w, ratings, err)
}

func (s *Server) listRewards(w http.ResponseWriter, r *http.Request) {
	rewards, err := s.store.ListRewards(r.Context())
	respond(w, rewards, err)
}

func (s *Server) createReward(w http.ResponseWriter, r *http.Request) {
	var reward domain.Reward
	if err := decodeJSON(r, &reward); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if reward.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	created, err := s.store.CreateReward(r.Context(), principal(r), reward)
	respondCreated(w, created, err)
}

func (s *Server) deleteReward(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(r.URL.Path, "api", "rewards")
	if !ok {
		writeError(w, http.StatusNotFound, "unknown reward")
		return
	}
	err := s.store.DeleteReward(r.Context(), principal(r), id)
	respond(w, map[string]string{"status": "deleted"}, err)
}

func (s *Server) exportBackup(w http.ResponseWriter, r *http.Request) {
	backup, err := s.store.ExportBackup(r.Context(), principal(r))
	if err != nil {
		respond(w, nil, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="familyquest-backup.json"`)
	writeJSON(w, http.StatusOK, backup)
}

func (s *Server) importBackup(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	defer r.Body.Close()

	var backup application.BackupData
	if err := decodeJSON(r, &backup); err != nil {
		writeError(w, http.StatusBadRequest, "invalid backup JSON")
		return
	}
	if err := s.store.ImportBackup(r.Context(), principal(r), backup); err != nil {
		respond(w, nil, err)
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
	if err != nil || id <= 0 {
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
	if err != nil || id <= 0 {
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
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func parseDate(value string) (time.Time, error) {
	if value == "" {
		return time.Now(), nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, domain.ErrInvalidInput
	}
	return parsed, nil
}

func decodeJSON(r *http.Request, value any) error {
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return domain.ErrInvalidInput
	}
	return nil
}

func respond(w http.ResponseWriter, payload any, err error) {
	respondStatus(w, payload, err, http.StatusOK)
}
func respondCreated(w http.ResponseWriter, payload any, err error) {
	respondStatus(w, payload, err, http.StatusCreated)
}
func respondStatus(w http.ResponseWriter, payload any, err error, success int) {
	if err == nil {
		writeJSON(w, success, payload)
		return
	}
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrInvalidRating), errors.Is(err, domain.ErrInvalidPINFormat), errors.Is(err, domain.ErrInvalidRole), errors.Is(err, domain.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrInvalidPIN), errors.Is(err, domain.ErrUnauthorized):
		status = http.StatusUnauthorized
	case errors.Is(err, domain.ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, domain.ErrConflict):
		status = http.StatusConflict
	}
	message := err.Error()
	if status == http.StatusInternalServerError {
		log.Printf("API error: %v", err)
		message = "internal server error"
	}
	writeError(w, status, message)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
