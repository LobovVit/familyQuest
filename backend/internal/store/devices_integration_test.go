package store

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lobov/familyquest/backend/internal/application"
	"github.com/lobov/familyquest/backend/internal/auth"
	"github.com/lobov/familyquest/backend/internal/domain"
	"github.com/lobov/familyquest/backend/internal/httpapi"
)

func TestTrustedDevicesHTTPIntegration(t *testing.T) {
	url := os.Getenv("FAMILYQUEST_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	s, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	t.Chdir("../..")
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `truncate participants,chores,rewards cascade`); err != nil {
		t.Fatal(err)
	}
	parent, err := s.CreateParticipant(ctx, domain.Participant{Name: "Parent", Role: domain.RoleParent}, "739281")
	if err != nil {
		t.Fatal(err)
	}
	child, err := s.CreateParticipant(ctx, domain.Participant{Name: "Child", Role: domain.RoleChild}, "391827")
	if err != nil {
		t.Fatal(err)
	}
	tokens, _ := auth.New("device-test-secret-at-least-32-characters", time.Hour)
	app := application.New(s, tokens)
	h := httpapi.NewServer(app, "https://family.test")
	call := func(method, path string, body any, cookie *http.Cookie, bearer, proof, origin string) *httptest.ResponseRecorder {
		var raw []byte
		if body != nil {
			raw, _ = json.Marshal(body)
		}
		r := httptest.NewRequest(method, "https://family.test"+path, strings.NewReader(string(raw)))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-FamilyQuest", "1")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		if proof != "" {
			r.Header.Set("X-FamilyQuest-Confirmation", proof)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	assert := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("status %d want %d: %s", w.Code, status, w.Body.String())
		}
	}
	login := func(id int64, pin, name string, parentID int64, parentPIN string, old *http.Cookie) *httptest.ResponseRecorder {
		return call("POST", "/api/session", map[string]any{"participantId": id, "pin": pin, "remember": true, "deviceName": name, "parentId": parentID, "parentPin": parentPIN}, old, "", "", "")
	}
	assert(login(child.ID, "391827", "Child iPad", 0, "", nil), 401)
	assert(login(child.ID, "391827", "Child iPad", child.ID, "391827", nil), 403)
	w := login(child.ID, "391827", "Child iPad", parent.ID, "739281", nil)
	assert(w, 200)
	childCookie := w.Result().Cookies()[0]
	if !childCookie.HttpOnly || !childCookie.Secure || childCookie.SameSite != http.SameSiteStrictMode || childCookie.MaxAge != 90*86400 {
		t.Fatal("insecure persistent cookie")
	}
	if strings.Contains(w.Body.String(), childCookie.Value) {
		t.Fatal("device secret exposed in JSON")
	}
	assert(call("GET", "/api/session", nil, childCookie, "", "", ""), 200)
	assert(call("GET", "/api/devices", nil, childCookie, "", "", ""), 403)
	assert(call("POST", "/api/session/logout", nil, childCookie, "", "", "https://evil.test"), 403)
	assert(call("POST", "/api/session/logout", nil, childCookie, "", "", "null"), 403)
	// The stored token is a digest, never the bearer cookie.
	var digest string
	if err = s.pool.QueryRow(ctx, `select token_hash from trusted_devices where participant_id=$1`, child.ID).Scan(&digest); err != nil {
		t.Fatal(err)
	}
	if digest == childCookie.Value || len(digest) != 64 {
		t.Fatal("raw secret persisted")
	}
	w = login(parent.ID, "739281", "Parent iPad", 0, "", nil)
	assert(w, 200)
	parentCookie := w.Result().Cookies()[0]
	// Force the second half of replacement to fail; neither half may commit.
	w = login(parent.ID, "739281", "Rollback sentinel", 0, "", nil)
	assert(w, 200)
	sentinel := w.Result().Cookies()[0]
	if _, err = s.pool.Exec(ctx, `alter table trusted_devices add constraint test_no_revoke check(name<>'Rollback sentinel' or revoked_at is null)`); err != nil {
		t.Fatal(err)
	}
	assert(login(parent.ID, "739281", "Must roll back", 0, "", sentinel), 500)
	assert(call("GET", "/api/session", nil, sentinel, "", "", ""), 200)
	var replacements int
	if err = s.pool.QueryRow(ctx, `select count(*) from trusted_devices where name='Must roll back'`).Scan(&replacements); err != nil || replacements != 0 {
		t.Fatal("replacement was partly committed", err)
	}
	if _, err = s.pool.Exec(ctx, `alter table trusted_devices drop constraint test_no_revoke`); err != nil {
		t.Fatal(err)
	}
	assert(call("POST", "/api/session/logout", map[string]string{}, sentinel, "", "", ""), 200)

	w = call("GET", "/api/devices", nil, parentCookie, "", "", "")
	assert(w, 200)
	var devices []domain.TrustedDevice
	if err = json.Unmarshal(w.Body.Bytes(), &devices); err != nil || len(devices) != 2 {
		t.Fatal("device list", err)
	}
	var childID string
	for _, d := range devices {
		if d.ParticipantID == child.ID {
			childID = d.ID
		}
		if d.ParticipantID == parent.ID && !d.Current {
			t.Fatal("current device missing")
		}
	}
	assert(call("DELETE", "/api/devices/"+childID, nil, parentCookie, "", "", ""), 403)
	assert(call("POST", "/api/session/confirm", map[string]string{"pin": "000000"}, parentCookie, "", "", ""), 401)
	proofFor := func(c *http.Cookie) string {
		w := call("POST", "/api/session/confirm", map[string]string{"pin": "739281"}, c, "", "", "")
		assert(w, 200)
		var v map[string]string
		json.Unmarshal(w.Body.Bytes(), &v)
		return v["proof"]
	}
	proof := proofFor(parentCookie)
	assert(call("GET", "/api/devices", nil, nil, proof, "", ""), 401) // Proof cannot become a login token.
	w = login(parent.ID, "739281", "Parent Mac", 0, "", nil)
	assert(w, 200)
	macCookie := w.Result().Cookies()[0]
	assert(call("DELETE", "/api/devices/"+childID, nil, macCookie, "", proof, ""), 403) // Bound to original device.
	assert(call("DELETE", "/api/devices/"+childID, nil, parentCookie, "", proof, ""), 200)
	assert(call("GET", "/api/session", nil, childCookie, "", "", ""), 401)
	assert(call("GET", "/api/session", nil, macCookie, "", "", ""), 200)
	// PIN change revokes every remembered device for that participant.
	w = login(child.ID, "391827", "Child phone", parent.ID, "739281", nil)
	assert(w, 200)
	childCookie = w.Result().Cookies()[0]
	assert(call("PUT", "/api/participants/"+fmt.Sprint(child.ID)+"/pin", map[string]string{"pin": "483927"}, parentCookie, "", "", ""), 403)
	assert(call("PUT", "/api/participants/"+fmt.Sprint(child.ID)+"/pin", map[string]string{"pin": "483927"}, parentCookie, "", proof, ""), 200)
	assert(call("GET", "/api/session", nil, childCookie, "", "", ""), 401)
	// Expiry cannot be extended by presenting an expired cookie.
	if _, err = s.pool.Exec(ctx, `update trusted_devices set expires_at=now()-interval '1 second' where name='Parent Mac'`); err != nil {
		t.Fatal(err)
	}
	assert(call("GET", "/api/session", nil, macCookie, "", "", ""), 401)
	// A temporary login removes the old binding without remembering the PIN.
	w = call("POST", "/api/session", map[string]any{"participantId": child.ID, "pin": "483927"}, parentCookie, "", "", "")
	assert(w, 200)
	assert(call("GET", "/api/session", nil, parentCookie, "", "", ""), 401)
	if w.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("temporary login retained device cookie")
	}
	// Restore must not resurrect device access or export any device tokens.
	w = login(parent.ID, "739281", "Restore test", 0, "", nil)
	assert(w, 200)
	parentCookie = w.Result().Cookies()[0]
	proof = proofFor(parentCookie)
	w = call("GET", "/api/backup", nil, parentCookie, "", "", "")
	assert(w, 200)
	if strings.Contains(w.Body.String(), "token_hash") || strings.Contains(w.Body.String(), "trusted_devices") {
		t.Fatal("backup exposed device credentials")
	}
	var backup application.BackupData
	if err = json.Unmarshal(w.Body.Bytes(), &backup); err != nil {
		t.Fatal(err)
	}
	assert(call("POST", "/api/backup", backup, parentCookie, "", "", ""), 403)
	assert(call("POST", "/api/backup", backup, parentCookie, "", proof, ""), 200)
	assert(call("GET", "/api/session", nil, parentCookie, "", "", ""), 401)
}
