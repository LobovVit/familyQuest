package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

// Embed the port so unexpected repository calls fail the test.
type behaviorRepository struct {
	application.Repository
	saved []domain.BehaviorRating
}

func (r *behaviorRepository) RateBehavior(_ context.Context, date time.Time, rater, target int64, rating int, comment string) (domain.BehaviorRating, error) {
	value := domain.BehaviorRating{RatedDate: date.Format("2006-01-02"), RaterParticipantID: rater, TargetParticipantID: target, Rating: rating}
	r.saved = append(r.saved, value)
	return value, nil
}

func TestBehaviorRatingAccessAndAuthor(t *testing.T) {
	for _, tc := range []struct {
		name, role string
		status     int
	}{
		{"child", domain.RoleChild, http.StatusCreated},
		{"parent", domain.RoleParent, http.StatusCreated},
		{"school", domain.RoleSchool, http.StatusForbidden},
		{"anonymous", "", http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, tokens := testServer(t)
			repo := &behaviorRepository{}
			handler := NewServer(application.New(repo, tokens), "*")
			req := httptest.NewRequest(http.MethodPost, "/api/behavior-ratings", strings.NewReader(`{"date":"2026-09-05","raterParticipantId":999,"targetParticipantId":3,"rating":4}`))
			if tc.role != "" {
				token, err := tokens.Issue(domain.Participant{ID: 2, Role: tc.role})
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Authorization", "Bearer "+token)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", response.Code, tc.status, response.Body.String())
			}
			if tc.status != http.StatusCreated {
				if len(repo.saved) != 0 {
					t.Fatal("unauthorized rating saved")
				}
				return
			}
			var got domain.BehaviorRating
			if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if len(repo.saved) != 1 || got.RaterParticipantID != 2 || got.TargetParticipantID != 3 || got.Rating != 4 || got.RatedDate != "2026-09-05" {
				t.Fatalf("incorrect saved rating: %+v", got)
			}
		})
	}
}
