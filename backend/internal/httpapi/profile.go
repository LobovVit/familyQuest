package httpapi

import (
	"github.com/lobov/familyquest/backend/internal/domain"
	"net/http"
	"strconv"
)

func (s *Server) profileRoutes() {
	s.mux.Handle("POST /api/reading/start", s.authorize(false, func(w http.ResponseWriter, r *http.Request) {
		var v domain.ReadingCompletion
		if decodeJSON(r, &v) != nil {
			respond(w, nil, domain.ErrInvalidInput)
			return
		}
		e := s.store.StartReading(r.Context(), principal(r), v)
		respond(w, map[string]string{"status": "started"}, e)
	}))

	s.mux.Handle("GET /api/learning-policy", s.authorize(false, func(w http.ResponseWriter, r *http.Request) {
		v, e := s.store.Policy(r.Context(), principal(r))
		respond(w, v, e)
	}))
	s.mux.Handle("GET /api/participants/{id}/learning", s.authorize(false, func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
		v, e := s.store.Profile(r.Context(), principal(r), id)
		respond(w, v, e)
	}))
	s.mux.Handle("PUT /api/participants/{id}/learning", s.authorize(true, func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
		var v domain.LearningProfile
		if e := decodeJSON(r, &v); e != nil {
			respond(w, nil, domain.ErrInvalidInput)
			return
		}
		e := s.store.SaveProfile(r.Context(), principal(r), id, v)
		respond(w, v, e)
	}))
}
