package httpapi

import (
	"github.com/lobov/familyquest/backend/internal/domain"
	"net/http"
)

func (s *Server) learningRoutes() {
	s.mux.Handle("POST /api/reading/complete", s.authorize(false, func(w http.ResponseWriter, r *http.Request) {
		var body domain.ReadingCompletion
		if !familyBody(w, r, &body) {
			return
		}
		v, err := s.store.CompleteReading(r.Context(), principal(r), body)
		respond(w, v, err)
	}))
	s.mux.Handle("POST /api/math/{id}/finish", s.authorize(false, func(w http.ResponseWriter, r *http.Request) {
		v, err := s.store.FinishMath(r.Context(), principal(r), r.PathValue("id"))
		respond(w, v, err)
	}))
	s.mux.Handle("GET /api/math", s.authorize(false, func(w http.ResponseWriter, r *http.Request) {
		v, err := s.store.MathSessions(r.Context(), principal(r))
		respond(w, v, err)
	}))
	s.mux.Handle("POST /api/math", s.authorize(false, func(w http.ResponseWriter, r *http.Request) {
		var settings domain.MathSettings
		if !familyBody(w, r, &settings) {
			return
		}
		v, err := s.store.StartMath(r.Context(), principal(r), settings)
		respondCreated(w, v, err)
	}))
	s.mux.Handle("POST /api/math/{id}/answers", s.authorize(false, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Index  int            `json:"index"`
			Values map[string]int `json:"values"`
		}
		if !familyBody(w, r, &body) {
			return
		}
		v, err := s.store.AnswerMath(r.Context(), principal(r), r.PathValue("id"), body.Index, body.Values)
		respond(w, v, err)
	}))
	s.mux.Handle("GET /api/activity-rewards", s.authorize(false, func(w http.ResponseWriter, r *http.Request) {
		v, err := s.store.ActivityRewards(r.Context(), principal(r))
		respond(w, v, err)
	}))
}
