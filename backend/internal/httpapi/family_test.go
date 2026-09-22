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

type familyRepository struct {
	application.Repository
	role  string
	saved []domain.FamilyEntry
}

func (r *familyRepository) GetParticipant(_ context.Context, id int64) (domain.Participant, error) {
	return domain.Participant{ID: id, Role: r.role, Active: true}, nil
}
func (r *familyRepository) ListFamilyEntries(context.Context) ([]domain.FamilyEntry, error) {
	return []domain.FamilyEntry{}, nil
}
func (r *familyRepository) SaveFamilyEntry(_ context.Context, e domain.FamilyEntry) (domain.FamilyEntry, error) {
	e.ID = 1
	e.Version = 1
	r.saved = append(r.saved, e)
	return e, nil
}
func TestFamilyHTTPAccessAndInput(t *testing.T) {
	for _, tc := range []struct {
		name, role, method, path, body string
		status                         int
	}{
		{"anonymous read", "", "GET", "/api/family", "", 401},
		{"school read", domain.RoleSchool, "GET", "/api/family", "", 403},
		{"child read", domain.RoleChild, "GET", "/api/family", "", 200},
		{"parent read", domain.RoleParent, "GET", "/api/family", "", 200},
		{"child sport", domain.RoleChild, "POST", "/api/family", `{"kind":"sport","draft":{"title":"Бег","date":"2026-09-19","participantIds":[2],"sport":{"activity":"Бег","minutes":30,"distanceKm":5,"exercises":[]}}}`, 201},
		{"child foreign sport", domain.RoleChild, "POST", "/api/family", `{"kind":"sport","draft":{"title":"Бег","date":"2026-09-19","participantIds":[3],"sport":{"activity":"Бег","minutes":30}}}`, 403},
		{"school sport", domain.RoleSchool, "POST", "/api/family", `{"kind":"sport","draft":{"title":"Бег","date":"2026-09-19","participantIds":[2],"sport":{"activity":"Бег","minutes":30}}}`, 403},
		{"sport negative duration", domain.RoleParent, "POST", "/api/family", `{"kind":"sport","draft":{"title":"Бег","date":"2026-09-19","participantIds":[2],"sport":{"activity":"Бег","minutes":-1}}}`, 400},
		{"child idea", domain.RoleChild, "POST", "/api/family", `{"kind":"adventure","draft":{"title":"Идея","date":"2026-09-19","steps":[{"title":"План"}]}}`, 201},
		{"forged author", domain.RoleChild, "POST", "/api/family", `{"kind":"memory","authorId":999,"draft":{"title":"A","date":"2026-09-19"}}`, 400},
		{"forged approval", domain.RoleChild, "POST", "/api/family", `{"kind":"proposal","approved":true,"draft":{"title":"A","date":"2026-09-19"}}`, 400},
		{"trailing JSON", domain.RoleParent, "POST", "/api/family", `{"kind":"memory","draft":{"title":"A","date":"2026-09-19"}} {}`, 400},
		{"invalid id", domain.RoleParent, "PUT", "/api/family/no", "{}", 404},
		{"missing action id", domain.RoleParent, "POST", "/api/family/-1/actions", "{}", 404},
		{"invalid date", domain.RoleParent, "POST", "/api/family", `{"kind":"memory","draft":{"title":"A","date":"2026-02-30"}}`, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := auth.New("family-test-secret-with-at-least-32-characters", time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			repo := &familyRepository{role: tc.role}
			handler := NewServer(application.New(repo, tokens), "*")
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.role != "" {
				token, err := tokens.Issue(domain.Participant{ID: 2, Role: tc.role})
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Authorization", "Bearer "+token)
			}
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if res.Code != tc.status {
				t.Fatalf("got %d want %d: %s", res.Code, tc.status, res.Body.String())
			}
			if tc.status == 201 {
				var got domain.FamilyEntry
				if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got.AuthorID != 2 || got.Approved || (got.Kind != "proposal" && got.Kind != "sport") {
					t.Fatalf("wrong actor or approval: %+v", got)
				}
			}
			if tc.status >= 400 && len(repo.saved) > 0 {
				t.Fatal("rejected input was persisted")
			}
		})
	}
}
