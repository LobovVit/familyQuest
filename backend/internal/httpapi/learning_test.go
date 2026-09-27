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

func (r learningRepository) CompleteReading(_ context.Context, owner int64, c domain.ReadingCompletion, now time.Time) (domain.ActivityReward, error) {
	return c.Reward(owner, 0, now), nil
}
func TestReadingHTTPBoundary(t *testing.T) {
	for _, role := range []string{"", domain.RoleParent, domain.RoleSchool, domain.RoleChild} {
		tokens, _ := auth.New("learning-secret-at-least-thirty-two-characters", time.Hour)
		handler := NewServer(application.New(learningRepository{role: role}, tokens), "*")
		req := httptest.NewRequest("POST", "/api/reading/complete", strings.NewReader(`{"id":"0123456789abcdef0123456789abcdef","level":"advanced"}`))
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
			want = 200
		}
		if response.Code != want {
			t.Fatalf("%s: %d %s", role, response.Code, response.Body.String())
		}
		if want == 200 {
			var r domain.ActivityReward
			if e := json.Unmarshal(response.Body.Bytes(), &r); e != nil || r.Stars != 12 || r.ParticipantID != 2 {
				t.Fatal(r, e)
			}
		}
	}
}

func TestReadingRejectsInvalidOrClientControlledRewards(t *testing.T) {
	tokens, _ := auth.New("learning-secret-at-least-thirty-two-characters", time.Hour)
	handler := NewServer(application.New(learningRepository{role: domain.RoleChild}, tokens), "*")
	token, _ := tokens.Issue(domain.Participant{ID: 2, Role: domain.RoleChild})
	for _, body := range []string{`{"id":"bad","level":"phrases"}`, `{"id":"0123456789abcdef0123456789abcdef","level":"unknown"}`, `{"id":"0123456789abcdef0123456789abcdef","level":"phrases","stars":999}`, `{"id":"0123456789abcdef0123456789abcdef","level":"phrases","participantId":3}`} {
		req := httptest.NewRequest("POST", "/api/reading/complete", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != 400 {
			t.Fatalf("%s: %d", body, response.Code)
		}
	}
}
