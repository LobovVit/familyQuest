package httpapi

import (
	"net/http"

	"github.com/lobov/familyquest/backend/internal/application"
	"github.com/lobov/familyquest/backend/internal/domain"
)

const HelperTasksPath = "/api/helper/tasks"
const HelperAccessPath = "/api/helper-access"

func (s *Server) helperRoutes() {
	s.mux.Handle("GET "+HelperAccessPath, s.authorize(true, s.listHelperAccess))
	s.mux.Handle("POST "+HelperAccessPath, s.authorize(true, s.createHelperAccess))
	s.mux.Handle("DELETE "+HelperAccessPath+"/", s.authorize(true, s.revokeHelperAccess))
}

func (s *Server) listHelperAccess(w http.ResponseWriter, r *http.Request) {
	result, err := s.store.ListHelperAccess(r.Context(), principal(r))
	respond(w, result, err)
}

func (s *Server) createHelperAccess(w http.ResponseWriter, r *http.Request) {
	if !sameSiteRequest(r) {
		respond(w, nil, domain.ErrForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var input struct {
		Label   string  `json:"label"`
		TaskIDs []int64 `json:"taskIds"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respond(w, nil, domain.ErrInvalidInput)
		return
	}
	access, token, err := s.store.CreateHelperAccess(r.Context(), principal(r), input.Label, input.TaskIDs)
	respondCreated(w, map[string]any{"access": access, "token": token}, err)
}

func (s *Server) revokeHelperAccess(w http.ResponseWriter, r *http.Request) {
	if !sameSiteRequest(r) {
		respond(w, nil, domain.ErrForbidden)
		return
	}
	id, ok := parseIDPath(r.URL.Path, "api", "helper-access")
	if !ok || id <= 0 {
		respond(w, nil, domain.ErrInvalidInput)
		return
	}
	if err := s.store.RevokeHelperAccess(r.Context(), principal(r), id); err != nil {
		respond(w, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *SaaS) readHelperTasks(w http.ResponseWriter, r *http.Request) {
	token, err := parseBearer(r.Header.Get("Authorization"))
	if err != nil {
		respond(w, nil, err)
		return
	}
	family, _, err := application.ParseHelperToken(token)
	if err != nil {
		respond(w, nil, domain.ErrUnauthorized)
		return
	}
	service, err := s.resolve(r.Context(), family)
	if err != nil {
		respond(w, nil, err)
		return
	}
	tasks, err := service.HelperTasks(r.Context(), token)
	if err != nil {
		respond(w, nil, err)
		return
	}
	subscription, err := s.platform.Subscription(r.Context(), family)
	if err == nil && (subscription.FamilyID != family || subscription.Status != "active") {
		err = domain.ErrUnauthorized
	}
	respond(w, tasks, err)
}
