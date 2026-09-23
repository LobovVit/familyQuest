package httpapi

import (
	"context"
	"encoding/json"
	"github.com/lobov/familyquest/backend/internal/application"
	"github.com/lobov/familyquest/backend/internal/auth"
	"github.com/lobov/familyquest/backend/internal/domain"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type learningRepository struct {
	application.Repository
	role string
}

func (r learningRepository) GetParticipant(_ context.Context, id int64) (domain.Participant, error) {
	return domain.Participant{ID: id, Role: r.role, Active: true}, nil
}
func (r learningRepository) CreateMathSession(_ context.Context, s domain.MathSession) (domain.MathSession, error) {
	return s, nil
}
func TestMathHTTPBoundary(t *testing.T) {
	for _, role := range []string{"", domain.RoleParent, domain.RoleSchool, domain.RoleChild} {
		tokens, _ := auth.New("learning-secret-at-least-thirty-two-characters", time.Hour)
		handler := NewServer(application.New(learningRepository{role: role}, tokens), "*")
		req := httptest.NewRequest("POST", "/api/math", strings.NewReader(`{"operation":"+","level":"easy","answerMode":"input","divisionMode":"full","allowNegative":false}`))
		if role != "" {
			token, _ := tokens.Issue(domain.Participant{ID: 2, Role: role})
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		want := 403
		if role == "" {
			want = 401
		}
		if role == domain.RoleChild {
			want = 201
		}
		if response.Code != want {
			t.Fatalf("role %s status %d: %s", role, response.Code, response.Body.String())
		}
		if want == 201 {
			var data map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
				t.Fatal(err)
			}
			if data["questions"] != nil || data["answers"] != nil || data["feedback"] != nil {
				t.Fatal("future answers exposed")
			}
			if data["question"] == nil {
				t.Fatal("missing question")
			}
		}
	}
}
