package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lobov/familyquest/backend/internal/application"
	"github.com/lobov/familyquest/backend/internal/auth"
	"github.com/lobov/familyquest/backend/internal/domain"
)

func testServer(t *testing.T) (http.Handler, *auth.Tokens) {
	t.Helper()
	tokens, e := auth.New("01234567890123456789012345678901", time.Hour)
	if e != nil {
		t.Fatal(e)
	}
	return NewServer(application.New(nil, tokens), "*"), tokens
}
func TestProtectedEndpointRequiresBearer(t *testing.T) {
	h, _ := testServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/chores", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", w.Code)
	}
}
func TestParentEndpointRejectsChild(t *testing.T) {
	h, tokens := testServer(t)
	token, _ := tokens.Issue(domain.Participant{ID: 2, Role: domain.RoleChild})
	r := httptest.NewRequest(http.MethodPost, "/api/participants", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got %d", w.Code)
	}
}
func TestCORSAllowsAuthorization(t *testing.T) {
	h, _ := testServer(t)
	r := httptest.NewRequest(http.MethodOptions, "/api/tasks", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Header().Get("Access-Control-Allow-Headers") != "Content-Type, Authorization" {
		t.Fatal("authorization header is not allowed")
	}
}
