package store

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestAccessServiceSeparatedPrivilegesAndSessions(t *testing.T) {
	dbURL := os.Getenv("FAMILYQUEST_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("disposable PostgreSQL required")
	}
	ctx := context.Background()
	root, e := Open(ctx, dbURL)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	t.Chdir("../..")
	p := NewPlatform(root)
	if e = p.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	suffix := fmt.Sprint(time.Now().UnixNano())
	ids := []int64{}
	defer func() {
		for _, id := range ids {
			for _, table := range []string{"audit", "payments", "accounts", "families"} {
				column := "family_id"
				if table == "families" {
					column = "id"
				}
				_, _ = root.pool.Exec(ctx, "delete from fq_platform."+table+" where "+column+"=$1", id)
			}
			_, _ = root.pool.Exec(ctx, "drop schema "+schemaFor(id)+" cascade")
			_, _ = root.pool.Exec(ctx, "drop role "+roleFor(id))
		}
	}()
	for _, name := range []string{"A", "B"} {
		id, err := p.Provision(ctx, name, name+suffix+"@example.test", "long-password-test", "Parent", "739281", "test", false)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	runtime := func(prefix string, configure func(context.Context, string) error) *Store {
		role := prefix + suffix
		if _, e = root.pool.Exec(ctx, "create role "+role+" login noinherit password 'runtime-test-only'"); e != nil {
			t.Fatal(e)
		}
		if e = configure(ctx, role); e != nil {
			t.Fatal(e)
		}
		u, _ := url.Parse(dbURL)
		u.User = url.UserPassword(role, "runtime-test-only")
		db, err := Open(ctx, u.String())
		if err != nil {
			t.Fatal(err)
		}
		return db
	}
	// Explicit defers keep the operator pool open while runtime roles are cleaned up.
	// Явная очистка сохраняет операторское соединение до удаления runtime-ролей.
	coreRole := "fq_core_" + suffix
	accessRole := "fq_access_" + suffix
	core := runtime("fq_core_", p.ConfigureCore)
	catalog := runtime("fq_access_", p.ConfigureAccess)
	defer func() {
		core.Close()
		catalog.Close()
		for _, role := range []string{coreRole, accessRole} {
			_, _ = root.pool.Exec(ctx, "drop owned by "+role)
			_, _ = root.pool.Exec(ctx, "drop role "+role)
		}
	}()
	cp, ap := NewPlatform(core), NewPlatform(catalog)
	if e = cp.CheckCoreRuntime(ctx); e != nil {
		t.Fatal(e)
	}
	if e = ap.CheckAccessRuntime(ctx); e != nil {
		t.Fatal(e)
	}
	// Fail startup if an operator accidentally grants catalog write privileges.
	// Ошибочная выдача записи в каталог должна блокировать запуск.
	if _, e = root.pool.Exec(ctx, "grant update on fq_platform.payments to "+accessRole); e != nil {
		t.Fatal(e)
	}
	if e = ap.CheckAccessRuntime(ctx); e == nil {
		t.Fatal("accepted writable catalog")
	}
	if _, e = root.pool.Exec(ctx, "revoke update on fq_platform.payments from "+accessRole); e != nil {
		t.Fatal(e)
	}
	for _, q := range []string{"select * from fq_platform.accounts", "select * from fq_platform.payments"} {
		if _, e = core.pool.Exec(ctx, q); e == nil {
			t.Fatal("core reads billing", q)
		}
	}
	for _, q := range []string{"select * from " + schemaFor(ids[0]) + ".participants", "set role " + roleFor(ids[0]), "update fq_platform.families set status='suspended'"} {
		if _, e = catalog.pool.Exec(ctx, q); e == nil {
			t.Fatal("access overprivileged", q)
		}
	}
	payment := ManualPayment{FamilyID: ids[0], Reference: "proxy-" + suffix, AmountMinor: 100, Currency: "BYN", StartsAt: time.Now().Add(-time.Hour), EndsAt: time.Now().Add(time.Hour), ChildLimit: 2, Operator: "test"}
	if e = p.RecordPayment(ctx, payment); e != nil {
		t.Fatal(e)
	}
	tokens, _ := auth.New(strings.Repeat("t", 40), time.Hour)
	resolve := func(ctx context.Context, id int64) (*application.Service, error) {
		repo, err := cp.Family(ctx, id)
		if err != nil {
			return nil, err
		}
		return application.NewForFamily(repo, tokens.ForFamily(id), id), nil
	}
	key := strings.Repeat("k", 48)
	origin := httptest.NewServer(httpapi.RequireAccess(httpapi.NewPrivateSaaS(CorePlatform{cp}, resolve, tokens, ""), key))
	defer origin.Close()
	gateway, err := access.New(ap, origin.URL, key, "")
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, token, body string, cookies []*http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://family.example"+path, strings.NewReader(body))
		r.Header.Set("X-FamilyQuest", "1")
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
	login := call("POST", "/api/account/login", "", fmt.Sprintf(`{"email":%q,"password":"long-password-test"}`, "A"+suffix+"@example.test"), nil)
	if login.Code != 200 {
		t.Fatal(login.Code, login.Body.String())
	}
	var session application.LoginResult
	if e = json.Unmarshal(login.Body.Bytes(), &session); e != nil {
		t.Fatal(e)
	}
	if session.Participant.FamilyID != ids[0] {
		t.Fatal("wrong family")
	}
	if w := call("GET", "/api/participants", session.Token, "", nil); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	sub := call("GET", "/api/subscription", session.Token, "", nil)
	var subscription domain.FamilySubscription
	_ = json.Unmarshal(sub.Body.Bytes(), &subscription)
	if sub.Code != 200 || !subscription.Paid || !subscription.Access || subscription.OwnerEmail != "" {
		t.Fatal(sub.Code, sub.Body.String())
	}
	remember := call("POST", "/api/session", session.Token, `{"participantId":1,"pin":"739281","remember":true,"deviceName":"Test device"}`, nil)
	if remember.Code != 200 {
		t.Fatal(remember.Code, remember.Body.String())
	}
	cookies := remember.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("missing remembered cookie")
	}
	if w := call("GET", "/api/session", "", "", cookies); w.Code != 200 {
		t.Fatal("cookie session", w.Code, w.Body.String())
	}
	if e = p.RefundPayment(ctx, payment.Reference, "test"); e != nil {
		t.Fatal(e)
	}
	if w := call("POST", "/api/participants", session.Token, `{"name":"Child","role":"child","pin":"391827"}`, nil); w.Code != 403 {
		t.Fatal("unpaid write", w.Code)
	}
	if w := call("GET", "/api/participants", session.Token, "", nil); w.Code != 200 {
		t.Fatal("unpaid read", w.Code)
	}
	if e = p.ResetAccount(ctx, "A"+suffix+"@example.test", "another-password-test", "test"); e != nil {
		t.Fatal(e)
	}
	if w := call("GET", "/api/participants", session.Token, "", nil); w.Code != 401 {
		t.Fatal("revoked token", w.Code)
	}
	if w := call("GET", "/api/session", "", "", cookies); w.Code != 401 {
		t.Fatal("revoked device", w.Code)
	}
	loginB := call("POST", "/api/account/login", "", fmt.Sprintf(`{"email":%q,"password":"long-password-test"}`, "B"+suffix+"@example.test"), nil)
	if loginB.Code != 200 {
		t.Fatal(loginB.Code, loginB.Body.String())
	}
	var sessionB application.LoginResult
	_ = json.Unmarshal(loginB.Body.Bytes(), &sessionB)
	if sessionB.Participant.FamilyID == session.Participant.FamilyID {
		t.Fatal("family crossover")
	}
	if e = p.Suspend(ctx, ids[1], true, "test"); e != nil {
		t.Fatal(e)
	}
	if w := call("GET", "/api/participants", sessionB.Token, "", nil); w.Code != 401 {
		t.Fatal("suspended family", w.Code)
	}
}
