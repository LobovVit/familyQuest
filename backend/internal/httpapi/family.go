package httpapi

import (
	"encoding/json"
	"github.com/lobov/familyquest/backend/internal/domain"
	"io"
	"net/http"
	"strconv"
)

func (s *Server) familyRoutes() {
	s.mux.Handle("GET /api/family", s.authorize(false, s.listFamily))
	s.mux.Handle("POST /api/family", s.authorize(false, s.createFamily))
	s.mux.Handle("PUT /api/family/{id}", s.authorize(false, s.editFamily))
	s.mux.Handle("POST /api/family/{id}/actions", s.authorize(false, s.familyAction))
}
func familyBody(w http.ResponseWriter, r *http.Request, v any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		writeError(w, 400, "Некорректные данные карточки")
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, 400, "Ожидается один JSON-объект")
		return false
	}
	return true
}
func (s *Server) listFamily(w http.ResponseWriter, r *http.Request) {
	v, err := s.store.FamilyEntries(r.Context(), principal(r))
	respond(w, v, err)
}
func (s *Server) createFamily(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Kind  string             `json:"kind"`
		Draft domain.FamilyDraft `json:"draft"`
	}
	if !familyBody(w, r, &request) {
		return
	}
	v, err := s.store.CreateFamilyEntry(r.Context(), principal(r), request.Kind, request.Draft)
	respondCreated(w, v, err)
}
func (s *Server) editFamily(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, 404, "Карточка не найдена")
		return
	}
	var request struct {
		Version int                `json:"version"`
		Draft   domain.FamilyDraft `json:"draft"`
	}
	if !familyBody(w, r, &request) {
		return
	}
	v, err := s.store.EditFamilyEntry(r.Context(), principal(r), id, request.Version, request.Draft)
	respond(w, v, err)
}
func (s *Server) familyAction(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, 404, "Карточка не найдена")
		return
	}
	var request domain.FamilyCommand
	if !familyBody(w, r, &request) {
		return
	}
	v, err := s.store.FamilyAction(r.Context(), principal(r), id, request)
	respond(w, v, err)
}
