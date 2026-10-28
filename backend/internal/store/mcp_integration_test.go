package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpAuthTransport struct{ token string }

func (a mcpAuthTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+a.token)
	return http.DefaultTransport.RoundTrip(r)
}
func TestMCPProfileDelegation(t *testing.T) {
	db := os.Getenv("FAMILYQUEST_TEST_DATABASE_URL")
	if db == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	s, e := Open(ctx, db)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	t.Chdir("../..")
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = s.pool.Exec(ctx, `truncate participants,chores,rewards cascade`); e != nil {
		t.Fatal(e)
	}
	parent, e := s.CreateParticipant(ctx, domain.Participant{Name: "Parent", Role: domain.RoleParent}, "739281")
	if e != nil {
		t.Fatal(e)
	}
	child, e := s.CreateParticipant(ctx, domain.Participant{Name: "Child", Role: domain.RoleChild}, "391827")
	if e != nil {
		t.Fatal(e)
	}
	school, e := s.CreateParticipant(ctx, domain.Participant{Name: "School", Role: domain.RoleSchool}, "482713")
	if e != nil {
		t.Fatal(e)
	}
	tokens, _ := auth.New(strings.Repeat("m", 40), time.Hour)
	app := application.New(s, tokens)
	actor := domain.Principal{ParticipantID: parent.ID, Role: parent.Role, SessionVersion: parent.SessionVersion}
	proof, e := app.ConfirmParent(ctx, actor, "739281")
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = app.CreateMCPGrant(ctx, actor, child.ID, "no proof", false, ""); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("missing proof", e)
	}
	grant, token, e := app.CreateMCPGrant(ctx, actor, child.ID, "Child agent", false, proof)
	if e != nil {
		t.Fatal(e)
	}
	_, readToken, e := app.CreateMCPGrant(ctx, actor, child.ID, "Read agent", true, proof)
	if e != nil {
		t.Fatal(e)
	}
	_, parentToken, e := app.CreateMCPGrant(ctx, actor, parent.ID, "Parent agent", false, proof)
	if e != nil {
		t.Fatal(e)
	}
	_, schoolToken, e := app.CreateMCPGrant(ctx, actor, school.ID, "School agent", false, proof)
	if e != nil {
		t.Fatal(e)
	}
	chore, e := app.CreateChore(ctx, actor, domain.Chore{Title: "Test chore", Schedule: "daily", TimeWindow: "morning", BenefitType: "self", ExecutionMode: "assigned", BaseValue: 1, ParticipantIDs: []int64{parent.ID, child.ID}})
	if e != nil {
		t.Fatal(e)
	}
	_ = chore
	tasks, e := app.ListTasks(ctx, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	var childTask, parentTask int64
	for _, v := range tasks {
		if v.ParticipantID == child.ID {
			childTask = v.ID
		}
		if v.ParticipantID == parent.ID {
			parentTask = v.ID
		}
	}
	if childTask == 0 || parentTask == 0 {
		t.Fatalf("missing tasks: %+v", tasks)
	}
	entry, e := app.CreateFamilyEntry(ctx, actor, "habit", domain.FamilyDraft{Title: "Habit", ParticipantIDs: []int64{child.ID}, Weekdays: []int{0, 1, 2, 3, 4, 5, 6}})
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(httpapi.NewServer(app, ""))
	defer server.Close()
	browserToken, _ := tokens.Issue(parent)
	request, _ := http.NewRequestWithContext(ctx, "POST", server.URL+"/api/mcp-grants", strings.NewReader(fmt.Sprintf(`{"participantId":%d,"name":"HTTP read default"}`, child.ID)))
	request.Header.Set("Authorization", "Bearer "+browserToken)
	request.Header.Set("X-FamilyQuest", "1")
	request.Header.Set("X-FamilyQuest-Confirmation", proof)
	request.Header.Set("Content-Type", "application/json")
	response, e := http.DefaultClient.Do(request)
	if e != nil {
		t.Fatal(e)
	}
	var issued struct {
		Grant domain.MCPGrant `json:"grant"`
		Token string          `json:"token"`
	}
	e = json.NewDecoder(response.Body).Decode(&issued)
	response.Body.Close()
	if e != nil || response.StatusCode != 201 || !issued.Grant.ReadOnly || issued.Grant.ParticipantID != child.ID || issued.Token == "" {
		t.Fatal("HTTP grant creation/default failed", response.StatusCode, e)
	}
	request, _ = http.NewRequestWithContext(ctx, "GET", server.URL+"/api/mcp-grants", nil)
	request.Header.Set("Authorization", "Bearer "+browserToken)
	response, e = http.DefaultClient.Do(request)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := io.ReadAll(response.Body)
	response.Body.Close()
	if e != nil || response.StatusCode != 200 || strings.Contains(string(raw), issued.Token) || strings.Contains(string(raw), `"token"`) {
		t.Fatal("grant list exposed credential", response.StatusCode, e)
	}
	request, _ = http.NewRequestWithContext(ctx, "DELETE", server.URL+"/api/mcp-grants/"+issued.Grant.ID, nil)
	request.Header.Set("Authorization", "Bearer "+browserToken)
	request.Header.Set("X-FamilyQuest", "1")
	response, e = http.DefaultClient.Do(request)
	if e != nil {
		t.Fatal(e)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("HTTP revoke failed", response.StatusCode)
	}
	if _, e = app.ParseToken(ctx, issued.Token); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("HTTP revocation ineffective", e)
	}

	connect := func(token string) *mcp.ClientSession {
		t.Helper()
		client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
		session, e := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/api/mcp", HTTPClient: &http.Client{Transport: mcpAuthTransport{token}}}, nil)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { session.Close() })
		return session
	}
	call := func(session *mcp.ClientSession, name string, args any, wantError bool) *mcp.CallToolResult {
		t.Helper()
		v, e := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if e != nil {
			if wantError {
				return nil
			}
			t.Fatal(e)
		}
		if v.IsError != wantError {
			t.Fatalf("%s: %+v", name, v)
		}
		return v
	}
	c := connect(token)
	ro := connect(readToken)
	pa := connect(parentToken)
	sc := connect(schoolToken)
	tools, e := c.ListTools(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range tools.Tools {
		if v.Name == "confirm_task" {
			t.Fatal("child can confirm")
		}
	}
	for _, sess := range []*mcp.ClientSession{ro, sc} {
		list, e := sess.ListTools(ctx, nil)
		if e != nil {
			t.Fatal(e)
		}
		for _, v := range list.Tools {
			if v.Name == "complete_task" || v.Name == "mark_activity" || v.Name == "confirm_task" {
				t.Fatal("unexpected write tool", v.Name)
			}
		}
	}
	result := call(c, "get_results", map[string]any{"date": time.Now().Format("2006-01-02")}, false)
	b, _ := json.Marshal(result.StructuredContent)
	var decoded application.MCPResults
	if e = json.Unmarshal(b, &decoded); e != nil {
		t.Fatal(e)
	}
	if len(decoded.Tasks) != 1 || decoded.Tasks[0].ParticipantID != child.ID {
		t.Fatalf("foreign child data: %s", b)
	}
	call(c, "get_results", map[string]any{"date": "yesterday"}, true)
	call(c, "get_learning_results", map[string]any{}, false)
	for _, owner := range []int64{parent.ID, child.ID} {
		_, e = app.CreateFamilyEntry(ctx, actor, "sport", domain.FamilyDraft{Title: "Sport", Date: time.Now().Format("2006-01-02"), ParticipantIDs: []int64{owner}, Sport: &domain.SportSession{Activity: "Walking", Minutes: 15}})
		if e != nil {
			t.Fatal(e)
		}
	}
	activities := call(c, "get_activities", map[string]any{}, false)
	rawActivities, _ := json.Marshal(activities.StructuredContent)
	var cards []application.MCPActivity
	if e = json.Unmarshal(rawActivities, &cards); e != nil {
		t.Fatal(e)
	}
	sports := 0
	for _, card := range cards {
		if card.Kind == "sport" {
			sports++
			if len(card.ParticipantIDs) != 1 || card.ParticipantIDs[0] != child.ID || card.Sport.Minutes != 15 {
				t.Fatal("foreign or missing sports data")
			}
		}
	}
	if sports != 1 {
		t.Fatal("sports not returned")
	}

	call(c, "get_learning_results", map[string]any{"participantId": parent.ID}, true)
	call(c, "complete_task", map[string]any{"taskId": parentTask}, true)
	call(c, "complete_task", map[string]any{"taskId": childTask}, false)
	call(pa, "confirm_task", map[string]any{"taskId": childTask, "rating": 5}, false)
	call(ro, "complete_task", map[string]any{"taskId": childTask}, true)
	call(sc, "get_activities", map[string]any{}, true)
	call(c, "mark_activity", map[string]any{"id": entry.ID, "version": entry.Version, "action": "checkin", "date": time.Now().Format("2006-01-02")}, false)
	call(c, "mark_activity", map[string]any{"id": entry.ID, "version": entry.Version, "action": "archive", "date": time.Now().Format("2006-01-02")}, true)
	for _, route := range []string{"/api/tasks", "/api/participants", "/api/session", "/api/mcp-grants"} {
		req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+route, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		res.Body.Close()
		if res.StatusCode != 403 {
			t.Fatal("MCP escaped to REST", route, res.StatusCode)
		}
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", server.URL+"/api/mcp", strings.NewReader(`{}`))
	req.Header.Set("Origin", "https://attacker.invalid")
	req.Header.Set("Authorization", "Bearer "+token)
	res, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal("invalid origin accepted", res.StatusCode)
	}
	if e = app.RemoveMCPGrant(ctx, actor, grant.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = app.ParseToken(ctx, token); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("revoked token accepted", e)
	}
	if _, e = c.ListTools(ctx, nil); e == nil {
		t.Fatal("existing MCP client survived revocation")
	}
	if _, e = s.UpdateParticipantPIN(ctx, child.ID, "291837"); e != nil {
		t.Fatal(e)
	}
	if _, e = app.ParseToken(ctx, readToken); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("PIN did not invalidate MCP", e)
	}

	for _, change := range []string{`active=false`, `role='parent'`} {
		if _, e = s.pool.Exec(ctx, "update participants set "+change+" where id=$1", school.ID); e != nil {
			t.Fatal(e)
		}
		if _, e = app.ParseToken(ctx, schoolToken); !errors.Is(e, domain.ErrUnauthorized) {
			t.Fatal("changed profile accepted", change, e)
		}
		if _, e = s.pool.Exec(ctx, `update participants set active=true,role='school' where id=$1`, school.ID); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = s.pool.Exec(ctx, `update mcp_grants set expires_at=now()-interval '1 second' where participant_id=$1`, school.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = app.ParseToken(ctx, schoolToken); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("expired grant accepted", e)
	}
	req, _ = http.NewRequestWithContext(ctx, "POST", server.URL+"/api/mcp", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+browserToken)
	res, e = http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal("browser token used as MCP grant", res.StatusCode)
	}
}

func TestMCPSaaSAccessIsolationAndExpiry(t *testing.T) {
	db := os.Getenv("FAMILYQUEST_TEST_DATABASE_URL")
	if db == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	root, e := Open(ctx, db)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	t.Chdir("../..")
	platform := NewPlatform(root)
	if e = platform.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	ids := []int64{}
	defer func() {
		for _, id := range ids {
			_, _ = root.pool.Exec(context.Background(), `delete from fq_platform.audit where family_id=$1`, id)
			_, _ = root.pool.Exec(context.Background(), `delete from fq_platform.accounts where family_id=$1`, id)
			_, _ = root.pool.Exec(context.Background(), `delete from fq_platform.families where id=$1`, id)
			_, _ = root.pool.Exec(context.Background(), "drop schema "+schemaFor(id)+" cascade")
			_, _ = root.pool.Exec(context.Background(), "drop role "+roleFor(id))
		}
	}()
	for _, name := range []string{"MCP A", "MCP B"} {
		id, e := platform.Provision(ctx, name, fmt.Sprintf("mcp-%d@example.test", time.Now().UnixNano()), "test-only-password", "Parent", "739281", "test", false)
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
		if _, e = root.pool.Exec(ctx, "update "+schemaFor(id)+".service_policy set access_until=now()+interval '1 day'"); e != nil {
			t.Fatal(e)
		}
	}
	tokens, _ := auth.New(strings.Repeat("f", 40), time.Hour)
	resolve := func(ctx context.Context, id int64) (*application.Service, error) {
		repo, e := platform.Family(ctx, id)
		if e != nil {
			return nil, e
		}
		return application.NewForFamily(repo, tokens.ForFamily(id), id), nil
	}
	appA, e := resolve(ctx, ids[0])
	if e != nil {
		t.Fatal(e)
	}
	appB, e := resolve(ctx, ids[1])
	if e != nil {
		t.Fatal(e)
	}
	people, e := appA.ListParticipants(ctx)
	if e != nil || len(people) != 1 {
		t.Fatal(people, e)
	}
	actor := domain.Principal{FamilyID: ids[0], ParticipantID: people[0].ID, Role: domain.RoleParent}
	proof, e := appA.ConfirmParent(ctx, actor, "739281")
	if e != nil {
		t.Fatal(e)
	}
	grant, token, e := appA.CreateMCPGrant(ctx, actor, actor.ParticipantID, "A parent", false, proof)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = appB.ParseToken(ctx, token); !errors.Is(e, domain.ErrUnauthorized) {
		t.Fatal("other family accepted credential", e)
	}
	if _, e = appB.MCPGrants(ctx, actor); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("other family accepted actor", e)
	}
	key := strings.Repeat("k", 40)
	origin := httptest.NewServer(httpapi.RequireAccess(httpapi.NewPrivateSaaS(platform, resolve, tokens, ""), key))
	defer origin.Close()
	g, e := access.New(platform, origin.URL, key, "")
	if e != nil {
		t.Fatal(e)
	}
	// A configured SSO boundary must accept ONLY validated MCP delegation without cookies.
	g.SSO = access.NewSSO(nil, nil)
	server := httptest.NewServer(g)
	defer server.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "integration", Version: "1"}, nil)
	session, e := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/api/mcp", HTTPClient: &http.Client{Transport: mcpAuthTransport{token}}}, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer session.Close()
	v, e := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_profile", Arguments: map[string]any{}})
	if e != nil || v.IsError {
		t.Fatal(v, e)
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/api/mcp-grants", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	res.Body.Close()
	if res.StatusCode != 401 && res.StatusCode != 403 {
		t.Fatal("REST escape through access", res.StatusCode)
	}
	if _, e = root.pool.Exec(ctx, "update "+schemaFor(ids[0])+".service_policy set access_until=now()-interval '1 day'"); e != nil {
		t.Fatal(e)
	}
	v, e = session.CallTool(ctx, &mcp.CallToolParams{Name: "get_results", Arguments: map[string]any{"date": time.Now().Format("2006-01-02")}})
	if e != nil || v.IsError {
		t.Fatal("expired history unreadable", v, e)
	}
	v, e = session.CallTool(ctx, &mcp.CallToolParams{Name: "complete_task", Arguments: map[string]any{"taskId": 1}})
	if e != nil || !v.IsError {
		t.Fatal("expired write allowed", v, e)
	}
	// Revocation remains possible through the use case even after expiry.
	if e = appA.RemoveMCPGrant(ctx, actor, grant.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = session.ListTools(ctx, nil); e == nil {
		t.Fatal("access accepted revoked grant")
	}
}
