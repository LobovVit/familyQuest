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

	"github.com/lobov/familyquest/backend/internal/access"
	"github.com/lobov/familyquest/backend/internal/application"
	"github.com/lobov/familyquest/backend/internal/auth"
	"github.com/lobov/familyquest/backend/internal/domain"
	"github.com/lobov/familyquest/backend/internal/httpapi"
)

func TestHelperPilotIsolation(t *testing.T) {
	url := os.Getenv("FAMILYQUEST_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("disposable postgres required")
	}
	ctx := context.Background()
	root, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(root.Close)
	t.Chdir("../..")

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	platform := NewPlatform(root)
	must(platform.Migrate(ctx))
	tokens, err := auth.New("synthetic-pilot-test-secret-only-123456", time.Hour)
	must(err)

	fixture := func(name string) (int64, string, string, string, []domain.Task) {
		t.Helper()
		email := fmt.Sprintf("%s-%d@example.test", name, time.Now().UnixNano())
		id, err := platform.Provision(ctx, name, email, "synthetic-password", name+" parent", "739281", "test", false)
		must(err)
		t.Cleanup(func() {
			for _, query := range []string{
				fmt.Sprintf("delete from fq_platform.audit where family_id=%d", id),
				fmt.Sprintf("delete from fq_platform.accounts where family_id=%d", id),
				fmt.Sprintf("delete from fq_platform.families where id=%d", id),
				"drop schema " + schemaFor(id) + " cascade",
				"drop role " + roleFor(id),
			} {
				if _, err := root.pool.Exec(ctx, query); err != nil {
					t.Error(err)
				}
			}
		})

		// Seed local access directly, without involving a payment provider.
		_, err = root.pool.Exec(ctx, "update "+schemaFor(id)+".service_policy set access_until=null")
		must(err)
		db, err := platform.Family(ctx, id)
		must(err)
		_, parentID, err := platform.Account(ctx, email, "synthetic-password")
		must(err)
		child, err := db.CreateParticipant(ctx, domain.Participant{Name: name + " child secret", Role: domain.RoleChild}, "391827")
		must(err)
		school, err := db.CreateParticipant(ctx, domain.Participant{Name: name + " school", Role: domain.RoleSchool}, "681827")
		must(err)
		for _, title := range []string{name + " shared", name + " private"} {
			chore, err := db.CreateChore(ctx, domain.Chore{Title: title, Description: "private description", Schedule: "daily", BaseValue: 7, Active: true})
			must(err)
			_, err = db.CreateAssignment(ctx, chore.ID, child.ID)
			must(err)
		}
		now := time.Now()
		tasks, err := db.ListTasks(ctx, now)
		must(err)
		if len(tasks) != 2 {
			t.Fatalf("expected two tasks, got %+v", tasks)
		}
		_, err = db.ListTasks(ctx, now.AddDate(0, 0, 1))
		must(err)
		app := application.NewForFamily(db, tokens.ForFamily(id), id)
		_, parentToken, err := app.Authenticate(ctx, parentID, "739281")
		must(err)
		_, childToken, err := app.Authenticate(ctx, child.ID, "391827")
		must(err)
		_, schoolToken, err := app.Authenticate(ctx, school.ID, "681827")
		must(err)
		return id, parentToken, childToken, schoolToken, tasks
	}

	a, parentA, childA, schoolA, tasksA := fixture("alpha")
	b, parentB, _, _, tasksB := fixture("beta")

	const key = "private-origin-test-secret-only-123456"
	resolve := func(ctx context.Context, family int64) (*application.Service, error) {
		db, err := platform.Family(ctx, family)
		if err != nil {
			return nil, err
		}
		return application.NewForFamily(db, tokens.ForFamily(family), family), nil
	}
	origin := httpapi.RequireAccess(httpapi.NewPrivateSaaS(platform, resolve, tokens, ""), key)
	server := httptest.NewServer(origin)
	defer server.Close()
	gateway, err := access.New(platform, server.URL, key, "")
	must(err)

	call := func(handler http.Handler, method, path, token, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("X-FamilyQuest", "1")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s %s: status %d, cache %q, want %d; %s", method, path, w.Code, w.Header().Get("Cache-Control"), want, w.Body.String())
		}
		return w
	}

	create := func(parent string, task domain.Task) (application.HelperAccess, string) {
		t.Helper()
		body := fmt.Sprintf(`{"label":"Synthetic helper","taskIds":[%d]}`, task.ID)
		w := call(gateway, http.MethodPost, httpapi.HelperAccessPath, parent, body, 201)
		var result struct {
			Access application.HelperAccess
			Token  string
		}
		must(json.Unmarshal(w.Body.Bytes(), &result))
		if result.Token == "" {
			t.Fatal("missing helper code")
		}
		return result.Access, result.Token
	}
	accessA, helperA := create(parentA, tasksA[1])
	_, helperB := create(parentB, tasksB[1])
	if tasksA[1].ID != tasksB[1].ID {
		t.Fatal("fixture must exercise overlapping tenant-local task identifiers")
	}

	for _, token := range []string{childA, schoolA} {
		call(gateway, http.MethodPost, httpapi.HelperAccessPath, token, `{"label":"invalid","taskIds":[1]}`, 403)
		call(gateway, http.MethodGet, httpapi.HelperAccessPath, token, "", 403)
		call(gateway, http.MethodDelete, fmt.Sprintf("%s/%d", httpapi.HelperAccessPath, accessA.ID), token, "", 403)
	}
	for _, token := range []string{parentA, childA, schoolA, ""} {
		call(gateway, http.MethodGet, httpapi.HelperTasksPath, token, "", 401)
	}
	for _, path := range []string{"/api/participants", "/api/tasks", "/api/chores", "/api/assignments", "/api/family", "/api/leaderboard", "/api/rewards", "/api/math", "/api/reading", "/api/backup", "/api/subscription", "/api/devices", "/api/mcp-grants", httpapi.HelperAccessPath} {
		call(gateway, http.MethodGet, path, helperA, "", 403)
	}
	call(gateway, http.MethodPost, httpapi.HelperTasksPath, helperA, "", 403)
	call(gateway, http.MethodPost, "/api/tasks/1/complete", helperA, "{}", 403)
	call(gateway, http.MethodPost, httpapi.HelperAccessPath, parentA, `{"label":"invalid task","taskIds":[1,2147483647]}`, 400)
	crossFamilyToken := strings.Replace(helperA, fmt.Sprintf(".%d.", a), fmt.Sprintf(".%d.", b), 1)
	call(gateway, http.MethodGet, httpapi.HelperTasksPath, crossFamilyToken, "", 401)

	assertTasks := func(token string, task domain.Task) {
		t.Helper()
		w := call(gateway, http.MethodGet, httpapi.HelperTasksPath+"?familyId="+fmt.Sprint(b), token, "", 200)
		expected, err := json.Marshal([]application.HelperTask{{Title: task.ChoreTitle, Date: task.DueDate}})
		must(err)
		if strings.TrimSpace(w.Body.String()) != string(expected) {
			t.Fatalf("unexpected helper projection: %s", w.Body.String())
		}
	}
	// The helper page does not use the owner SSO session.
	gateway.SSO = &access.SSO{}
	assertTasks(helperA, tasksA[1])
	assertTasks(helperB, tasksB[1])
	gateway.SSO = nil

	parentList := call(gateway, http.MethodGet, httpapi.HelperAccessPath, parentA, "", 200)
	var grants []application.HelperAccess
	must(json.Unmarshal(parentList.Body.Bytes(), &grants))
	if len(grants) != 1 || grants[0].ID != accessA.ID || strings.Contains(parentList.Body.String(), helperA) {
		t.Fatalf("unexpected access list: %s", parentList.Body.String())
	}

	// Revocation remains available when regular writes have expired.
	_, err = root.pool.Exec(ctx, "update "+schemaFor(a)+".service_policy set access_until=now()")
	must(err)
	call(gateway, http.MethodDelete, fmt.Sprintf("%s/%d", httpapi.HelperAccessPath, accessA.ID), parentA, "", 204)
	call(gateway, http.MethodGet, httpapi.HelperTasksPath, helperA, "", 401)
	assertTasks(helperB, tasksB[1])
	_, err = root.pool.Exec(ctx, "update "+schemaFor(b)+".helper_access set expires_at=now()")
	must(err)
	call(gateway, http.MethodGet, httpapi.HelperTasksPath, helperB, "", 401)
}
