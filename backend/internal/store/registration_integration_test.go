package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lobov/familyquest/backend/internal/access"
	"github.com/lobov/familyquest/backend/internal/application"
	"github.com/lobov/familyquest/backend/internal/auth"
	"github.com/lobov/familyquest/backend/internal/domain"
	"github.com/lobov/familyquest/backend/internal/httpapi"
	"github.com/lobov/familyquest/backend/internal/identity"
	"github.com/lobov/familyquest/backend/internal/provisioning"
	"github.com/lobov/familyquest/backend/internal/testoidc"
)

type registrationAdapter struct {
	p    *Platform
	role string
}

func (a registrationAdapter) RegisterFamily(ctx context.Context, i, s string, v domain.FamilyRegistration) (int64, error) {
	return a.p.RegisterFamily(ctx, i, s, v, a.role)
}

func TestSelfRegistrationEndToEnd(t *testing.T) {
	databaseURL := os.Getenv("FAMILYQUEST_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("disposable postgres required")
	}
	t.Chdir("../..")
	ctx := context.Background()
	root, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	p := NewPlatform(root)
	if err = p.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprint(time.Now().UnixNano())
	coreRole, accessRole := "fq_core_reg_"+suffix, "fq_access_reg_"+suffix
	ids := []int64{}
	defer func() {
		for _, id := range ids {
			_, _ = root.pool.Exec(ctx, `delete from fq_platform.audit where family_id=$1`, id)
			_, _ = root.pool.Exec(ctx, `delete from fq_platform.accounts where family_id=$1`, id)
			_, _ = root.pool.Exec(ctx, `delete from fq_platform.families where id=$1`, id)
			_, _ = root.pool.Exec(ctx, "drop schema "+schemaFor(id)+" cascade")
			_, _ = root.pool.Exec(ctx, "drop role "+roleFor(id))
		}
		for _, role := range []string{coreRole, accessRole} {
			_, _ = root.pool.Exec(ctx, "drop owned by "+role)
			_, _ = root.pool.Exec(ctx, "drop role "+role)
		}
	}()
	for _, role := range []string{coreRole, accessRole} {
		if _, err = root.pool.Exec(ctx, "create role "+role+" login noinherit password 'registration-test-only'"); err != nil {
			t.Fatal(err)
		}
	}
	if err = p.ConfigureCore(ctx, coreRole); err != nil {
		t.Fatal(err)
	}
	if err = p.ConfigureAccess(ctx, accessRole); err != nil {
		t.Fatal(err)
	}
	openRole := func(role string) *Store {
		u, _ := url.Parse(databaseURL)
		u.User = url.UserPassword(role, "registration-test-only")
		db, err := Open(ctx, u.String())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(db.Close)
		return db
	}
	core, catalog := NewPlatform(openRole(coreRole)), NewPlatform(openRole(accessRole))
	provider := testoidc.New(t, nil)
	handler, err := provisioning.Handler(registrationAdapter{p, coreRole}, provider.Server.URL, strings.Repeat("p", 40))
	if err != nil {
		t.Fatal(err)
	}
	provisioner := httptest.NewServer(handler)
	defer provisioner.Close()
	client, err := provisioning.NewClient(provisioner.URL, strings.Repeat("p", 40))
	if err != nil {
		t.Fatal(err)
	}
	tokens, _ := auth.New(strings.Repeat("t", 40), time.Hour)
	resolve := func(ctx context.Context, id int64) (*application.Service, error) {
		repo, err := core.Family(ctx, id)
		if err != nil {
			return nil, err
		}
		return application.NewForFamily(repo, tokens.ForFamily(id), id), nil
	}
	coreKey := strings.Repeat("c", 40)
	origin := httptest.NewServer(httpapi.RequireAccess(httpapi.NewPrivateSaaS(CorePlatform{core}, resolve, tokens, ""), coreKey))
	defer origin.Close()
	gateway, err := access.New(catalog, origin.URL, coreKey, "https://family.example")
	if err != nil {
		t.Fatal(err)
	}
	login, err := identity.New(ctx, identity.Config{Issuer: provider.Server.URL, ClientID: "familyquest", ClientSecret: "test-secret", RedirectURL: "https://family.example/api/account/callback"})
	if err != nil {
		t.Fatal(err)
	}
	gateway.SSO = access.NewSSO(login, catalog)
	gateway.Registrar = client
	call := func(path, body, token, source string, cookies []*http.Cookie) *httptest.ResponseRecorder {
		method := "POST"
		if path == "/api/family/name" {
			method = "PUT"
		}
		if body == "" {
			method = "GET"
		}
		r := httptest.NewRequest(method, "https://family.example"+path, strings.NewReader(body))
		r.Header.Set("X-FamilyQuest", "1")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", source)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		for _, c := range cookies {
			r.AddCookie(c)
		}
		w := httptest.NewRecorder()
		gateway.ServeHTTP(w, r)
		return w
	}
	same := "https://family.example"
	start := call("/api/account/authorize", "", "", same, nil)
	code, state := provider.Code(t, start.Header().Get("Location"))
	callback := call("/api/account/callback?code="+code+"&state="+state, "", "", same, start.Result().Cookies())
	entry := call("/api/account/exchange", "{}", "", same, callback.Result().Cookies())
	if entry.Code != 200 || !strings.Contains(entry.Body.String(), `"onboarding":true`) {
		t.Fatal("onboarding", entry.Code, entry.Body.String())
	}
	cookies := entry.Result().Cookies()
	restore := call("/api/account/restore", "{}", "", same, cookies)
	if restore.Code != 200 || !strings.Contains(restore.Body.String(), `"onboarding":true`) {
		t.Fatal("restore onboarding", restore.Code)
	}
	if w := call("/api/participants", "", "", same, cookies); w.Code != 401 {
		t.Fatal("onboarding leaked data", w.Code)
	}
	if w := call("/api/account/profile", `{"participantId":1,"pin":"739281"}`, "", same, cookies); w.Code != 403 {
		t.Fatal("profile before creation", w.Code)
	}
	body := `{"name":"  Соколовы  ","parentName":" Анна ","pin":"739281"}`
	for _, tc := range []struct {
		body, source string
		cookies      []*http.Cookie
		status       int
	}{
		{body, same, nil, 401}, {body, "https://attacker.example", cookies, 403},
		{`{"name":"Семья","parentName":"Анна","pin":"739281","subject":"other"}`, same, cookies, 400},
		{`{"name":"  ","parentName":"Анна","pin":"739281"}`, same, cookies, 400},
	} {
		if w := call("/api/account/family", tc.body, "", tc.source, tc.cookies); w.Code != tc.status {
			t.Fatal("registration boundary", w.Code, tc.status, w.Body.String())
		}
	}
	created := call("/api/account/family", body, "", same, cookies)
	if created.Code != 200 || strings.Contains(created.Body.String(), `"token"`) {
		t.Fatal("create", created.Code, created.Body.String())
	}
	var profiles []domain.Participant
	if json.Unmarshal(created.Body.Bytes(), &profiles) != nil || len(profiles) != 1 || profiles[0].Name != "Анна" || profiles[0].Role != "parent" {
		t.Fatal("new profiles", created.Body.String())
	}
	family := profiles[0].FamilyID
	ids = append(ids, family)
	linked, owner, err := p.Identity(ctx, provider.Server.URL, "shared-person")
	if err != nil || linked != family || owner != profiles[0].ID {
		t.Fatal("identity binding", linked, owner, err)
	}
	cookies = created.Result().Cookies()
	if w := call("/api/account/family", body, "", same, cookies); w.Code != 200 {
		t.Fatal("retry", w.Code, w.Body.String())
	}
	if w := call("/api/account/restore", "{}", "", same, cookies); w.Code != 200 || strings.Contains(w.Body.String(), "onboarding") {
		t.Fatal("created restore", w.Code)
	}
	denied := call("/api/account/profile", fmt.Sprintf(`{"participantId":%d,"pin":"000000"}`, owner), "", same, cookies)
	if denied.Code == 200 {
		t.Fatal("wrong PIN accepted")
	}
	selected := call("/api/account/profile", fmt.Sprintf(`{"participantId":%d,"pin":"739281"}`, owner), "", same, cookies)
	var session application.LoginResult
	if selected.Code != 200 || json.Unmarshal(selected.Body.Bytes(), &session) != nil || session.Token == "" {
		t.Fatal("select", selected.Code, selected.Body.String())
	}
	subResponse := call("/api/subscription", "", session.Token, same, cookies)
	var sub domain.FamilySubscription
	if subResponse.Code != 200 || json.Unmarshal(subResponse.Body.Bytes(), &sub) != nil || sub.Name != "Соколовы" || sub.Paid || sub.Access || sub.Plan != "registered" {
		t.Fatal("subscription", subResponse.Code, subResponse.Body.String())
	}
	// Name changes are parent-only and stay available for an unpaid family.
	for _, tc := range []struct {
		body, source, token string
		status              int
	}{
		{`{"name":"  Новое название  "}`, same, session.Token, 200},
		{`{"name":" "}`, same, session.Token, 400},
		{`{"name":"Hacked","familyId":999}`, same, session.Token, 400},
		{`{"name":"Hacked"}`, "https://attacker.example", session.Token, 403},
		{`{"name":"Hacked"}`, same, "", 401},
	} {
		w := call("/api/family/name", tc.body, tc.token, tc.source, cookies)
		if w.Code != tc.status {
			t.Fatal("rename boundary", w.Code, tc.status, w.Body.String())
		}
	}
	subResponse = call("/api/subscription", "", session.Token, same, cookies)
	if json.Unmarshal(subResponse.Body.Bytes(), &sub) != nil || sub.Name != "Новое название" {
		t.Fatal("rename not persisted", subResponse.Body.String())
	}
	// Parent rights are checked by Core even for valid child sessions.
	var child int64
	err = root.pool.QueryRow(ctx, "insert into "+schemaFor(family)+".participants(name,role,pin_hash,pin_code) values('Child','child','unused',null) returning id").Scan(&child)
	if err != nil {
		t.Fatal(err)
	}
	childToken, err := tokens.ForFamily(family).Issue(domain.Participant{ID: child, FamilyID: family, Role: domain.RoleChild, SessionVersion: 0})
	if err != nil {
		t.Fatal(err)
	}
	if w := call("/api/family/name", `{"name":"Child rename"}`, childToken, same, cookies); w.Code != 403 {
		t.Fatal("child renamed family", w.Code, w.Body.String())
	}
	if _, err = core.root.pool.Exec(ctx, `update fq_platform.families set plan='forged' where id=$1`, family); err == nil {
		t.Fatal("rename grant permits billing changes")
	}
	if err = core.CheckCoreRuntime(ctx); err != nil {
		t.Fatal(err)
	}
	if err = catalog.CheckAccessRuntime(ctx); err != nil {
		t.Fatal(err)
	}
	// Migration remains repeatable with NULL OIDC credentials and legacy password accounts.
	legacy, err := p.Provision(ctx, "Legacy", "legacy"+suffix+"@example.test", "long-password-test", "Parent", "739281", "test", false)
	if err != nil {
		t.Fatal(err)
	}
	ids = append(ids, legacy)
	if err = p.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if id, _, err := p.Account(ctx, "legacy"+suffix+"@example.test", "long-password-test"); err != nil || id != legacy {
		t.Fatal("legacy account lost", err)
	}
	if registry, err := p.Subscription(ctx, family); err != nil || registry.OwnerEmail != "" {
		t.Fatal("OIDC owner email", err)
	}
	// Concurrent retries converge; changed fields never rename/reset an existing family.
	input := domain.FamilyRegistration{Name: "Other name", ParentName: "Other parent", PIN: "654321"}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := p.RegisterFamily(ctx, provider.Server.URL, "shared-person", input, coreRole)
			if err != nil || id != family {
				t.Errorf("retry id=%d err=%v", id, err)
			}
		}()
	}
	wg.Wait()
	if v, err := p.Subscription(ctx, family); err != nil || v.Name != "Новое название" {
		t.Fatal("registration retry renamed family", err)
	}
	var count int
	if err = root.pool.QueryRow(ctx, `select count(*) from fq_platform.identities where issuer=$1 and subject='shared-person'`, provider.Server.URL).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate identities", count, err)
	}
	createdIDs := make(chan int64, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := p.RegisterFamily(ctx, provider.Server.URL, "other-person", input, coreRole)
			if err != nil {
				t.Error(err)
			}
			createdIDs <- id
		}()
	}
	wg.Wait()
	close(createdIDs)
	var other int64
	for id := range createdIDs {
		if id <= 0 || id == family || (other != 0 && other != id) {
			t.Fatal("concurrent creation", id, other)
		}
		other = id
	}
	ids = append(ids, other)
	// A tenant cannot read the newly created neighbor schema.
	scoped, err := core.Family(ctx, family)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = scoped.db.Exec(ctx, "select * from "+schemaFor(other)+".participants"); err == nil {
		t.Fatal("tenant crossover")
	}
	// An invalid Core grant fails atomically, including the catalog entry.
	if _, err = p.RegisterFamily(ctx, provider.Server.URL, "bad-role", input, accessRole); err == nil {
		t.Fatal("Access granted a tenant role")
	}
	if _, err = p.PortalFamily(ctx, provider.Server.URL, "bad-role"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatal("partial registration", err)
	}
	_, err = root.pool.Exec(ctx, `create function fq_platform.reject_test_registration() returns trigger language plpgsql as $$ begin if new.action='self-register' then raise exception 'synthetic failure'; end if; return new; end $$; create trigger reject_test_registration before insert on fq_platform.audit for each row execute function fq_platform.reject_test_registration()`)
	if err != nil {
		t.Fatal(err)
	}
	defer root.pool.Exec(ctx, `drop trigger if exists reject_test_registration on fq_platform.audit; drop function if exists fq_platform.reject_test_registration()`)
	if _, err = p.RegisterFamily(ctx, provider.Server.URL, "rollback-person", input, coreRole); err == nil {
		t.Fatal("expected audit failure")
	}
	var rolledBackID int64
	if err = root.pool.QueryRow(ctx, `select last_value from fq_platform.families_id_seq`).Scan(&rolledBackID); err != nil {
		t.Fatal(err)
	}
	var residue bool
	if err = root.pool.QueryRow(ctx, `select exists(select 1 from fq_platform.families where id=$1) or exists(select 1 from pg_namespace where nspname=$2) or exists(select 1 from pg_roles where rolname=$3)`, rolledBackID, schemaFor(rolledBackID), roleFor(rolledBackID)).Scan(&residue); err != nil || residue {
		t.Fatal("partial tenant after rollback", residue, err)
	}
	if _, err = p.PortalFamily(ctx, provider.Server.URL, "rollback-person"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatal("partial identity after rollback", err)
	}
	if _, err = root.pool.Exec(ctx, `drop trigger reject_test_registration on fq_platform.audit; drop function fq_platform.reject_test_registration()`); err != nil {
		t.Fatal(err)
	}
	if err = p.Suspend(ctx, family, true, "test"); err != nil {
		t.Fatal(err)
	}
	if _, err = p.RegisterFamily(ctx, provider.Server.URL, "shared-person", input, coreRole); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal("suspended re-registration", err)
	}
	if w := call("/api/account/family", body, "", same, cookies); w.Code == 200 {
		t.Fatal("suspended access")
	}
}
