package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lobov/familyquest/backend/internal/application"
	"github.com/lobov/familyquest/backend/internal/auth"
	"github.com/lobov/familyquest/backend/internal/domain"
	"github.com/lobov/familyquest/backend/internal/httpapi"
	"net/http"
	"net/http/httptest"
	urlpkg "net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSaaSIsolationBillingAndBackup(t *testing.T) {
	url := os.Getenv("FAMILYQUEST_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("disposable postgres required")
	}
	ctx := context.Background()
	root, e := Open(ctx, url)
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
			_, _ = root.pool.Exec(ctx, `delete from fq_platform.audit where family_id=$1`, id)
			_, _ = root.pool.Exec(ctx, `delete from fq_platform.payments where family_id=$1`, id)
			_, _ = root.pool.Exec(ctx, `delete from fq_platform.accounts where family_id=$1`, id)
			_, _ = root.pool.Exec(ctx, `delete from fq_platform.families where id=$1`, id)
			_, _ = root.pool.Exec(ctx, "drop schema "+schemaFor(id)+" cascade")
			_, _ = root.pool.Exec(ctx, "drop role "+roleFor(id))
		}
	}()
	for _, name := range []string{"A", "B"} {
		id, e := p.Provision(ctx, name, name+suffix+"@example.test", "long-password-test", "Parent", "739281", "test", false)
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
	}
	runtimeRole := "fq_runtime_" + suffix
	if _, e = root.pool.Exec(ctx, "create role "+runtimeRole+" login noinherit password 'runtime-test-only'"); e != nil {
		t.Fatal(e)
	}
	defer func() {
		_, _ = root.pool.Exec(ctx, "drop owned by "+runtimeRole)
		_, _ = root.pool.Exec(ctx, "drop role "+runtimeRole)
	}()
	if e = p.ConfigureRuntime(ctx, runtimeRole); e != nil {
		t.Fatal(e)
	}
	u, e := urlpkg.Parse(url)
	if e != nil {
		t.Fatal(e)
	}
	u.User = urlpkg.UserPassword(runtimeRole, "runtime-test-only")
	q := u.Query()
	q.Set("pool_max_conns", "1")
	u.RawQuery = q.Encode()
	runtime, e := Open(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	defer runtime.Close()
	rp := NewPlatform(runtime)
	if e = rp.CheckRuntime(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = runtime.pool.Exec(ctx, `update fq_platform.families set plan='forged'`); e == nil {
		t.Fatal("runtime changed billing")
	}
	a, e := rp.Family(ctx, ids[0])
	if e != nil {
		t.Fatal(e)
	}
	b, e := rp.Family(ctx, ids[1])
	if e != nil {
		t.Fatal(e)
	}
	payment := ManualPayment{FamilyID: ids[0], Reference: "test-" + suffix, AmountMinor: 10000, Currency: "BYN", StartsAt: time.Now().Add(-time.Minute).Truncate(time.Microsecond), EndsAt: time.Now().Add(24 * time.Hour).Truncate(time.Microsecond), ChildLimit: 1, Operator: "test"}
	if e = p.RecordPayment(ctx, payment); e != nil {
		t.Fatal(e)
	}
	if e = p.RecordPayment(ctx, payment); e != nil {
		t.Fatal("retry", e)
	}
	bad := payment
	bad.AmountMinor++
	if !errors.Is(p.RecordPayment(ctx, bad), domain.ErrConflict) {
		t.Fatal("changed retry accepted")
	}
	bv, e := p.Subscription(ctx, ids[1])
	if e != nil || bv.Access || bv.Paid {
		t.Fatal("unpaid family", bv, e)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, e := a.CreateParticipant(ctx, domain.Participant{Name: fmt.Sprint("Child", i), Role: "child"}, "391827")
			results <- e
		}(i)
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		} else if !errors.Is(e, domain.ErrConflict) {
			t.Fatal(e)
		}
	}
	if success != 1 {
		t.Fatal("seat race", success)
	}
	for _, query := range []string{"select * from " + schemaFor(ids[1]) + ".participants", "select * from fq_platform.accounts", "update service_policy set child_limit=100", "truncate service_policy"} {
		if _, e = a.db.Exec(ctx, query); e == nil {
			t.Fatal("cross-boundary SQL accepted", query)
		}
	}
	tx, err := a.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `select unknown_column from participants`); err == nil {
		t.Fatal("expected transaction failure")
	}
	rollback(tx)
	var role string
	if err = runtime.pool.QueryRow(ctx, `select current_user`).Scan(&role); err != nil || role != runtimeRole {
		t.Fatal("role leaked to pool", role, err)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	rows, err := a.db.Query(cancelCtx, `select id from participants`)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	rows.Close()
	if _, err = b.ListParticipants(ctx); err != nil {
		t.Fatal("cancelled transaction poisoned pool", err)
	}
	people, e := a.ListParticipants(ctx)
	if e != nil || len(people) != 2 {
		t.Fatal(people, e)
	}
	child := people[1]
	bp, e := b.ListParticipants(ctx)
	if e != nil || len(bp) != 1 {
		t.Fatal(bp, e)
	}
	if e = a.SaveLearningProfile(ctx, child.ID, domain.LearningProfile{BirthDate: "2020-09-26"}); e != nil {
		t.Fatal(e)
	}
	settings := domain.MathSettings{Operation: "+", Level: "hard", AnswerMode: "input", DivisionMode: "full"}
	if _, e = a.CreateMathSession(ctx, domain.NewMathSession("policy-test", child.ID, settings, time.Now())); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("age bypass", e)
	}
	if e = a.SaveLearningProfile(ctx, child.ID, domain.LearningProfile{BirthDate: "2020-09-26", ReadingLevel: "advanced"}); e != nil {
		t.Fatal(e)
	}
	reading := domain.ReadingCompletion{ID: strings.Repeat("a", 32), Level: "advanced"}
	if e = a.StartReading(ctx, child.ID, reading, time.Now()); e != nil {
		t.Fatal(e)
	}
	if e = a.SaveLearningProfile(ctx, child.ID, domain.LearningProfile{BirthDate: "2020-09-26"}); e != nil {
		t.Fatal(e)
	}
	if _, e = a.CompleteReading(ctx, child.ID, reading, time.Now()); e != nil {
		t.Fatal("reading snapshot", e)
	}
	forged := reading
	forged.ID = strings.Repeat("b", 32)
	if _, e = a.CompleteReading(ctx, child.ID, forged, time.Now()); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("completion without start", e)
	}
	backup, e := a.ExportBackup(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if backup.FamilyID != ids[0] {
		t.Fatal("missing backup boundary")
	}
	if e = b.ImportBackup(ctx, backup); e == nil {
		t.Fatal("foreign backup accepted")
	}
	if e = a.ImportBackup(ctx, backup); e != nil {
		t.Fatal("own restore", e)
	}
	after, e := b.ListParticipants(ctx)
	if e != nil || len(after) != 1 || after[0].Name != bp[0].Name {
		t.Fatal("restore damaged B", after, e)
	}
	av, e := p.Subscription(ctx, ids[0])
	if e != nil || !av.Paid || !av.Access || av.ChildLimit != 1 {
		t.Fatal("restore changed billing", av, e)
	}
	tokens, _ := auth.New("saas-test-secret-at-least-32-characters", time.Hour)
	resolve := func(ctx context.Context, id int64) (*application.Service, error) {
		repo, e := rp.Family(ctx, id)
		if e != nil {
			return nil, e
		}
		return application.NewForFamily(repo, tokens.ForFamily(id), id), nil
	}
	handler := httpapi.NewSaaS(rp, resolve, tokens, "")
	call := func(method, path, token, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("X-FamilyQuest", "1")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, path := range []string{"/api/participants", "/api/leaderboard", "/api/family/overview"} {
		if w := call("GET", path, "", ""); w.Code != 401 {
			t.Fatal("public data", path, w.Code)
		}
	}
	login := call("POST", "/api/account/login", "", fmt.Sprintf(`{"email":%q,"password":"long-password-test"}`, "A"+suffix+"@example.test"))
	if login.Code != 200 {
		t.Fatal(login.Code, login.Body.String())
	}
	var session application.LoginResult
	if e = json.Unmarshal(login.Body.Bytes(), &session); e != nil {
		t.Fatal(e)
	}
	oldToken, _ := tokens.Issue(domain.Participant{ID: 1, Role: domain.RoleParent})
	if w := call("GET", "/api/participants", oldToken, ""); w.Code != 401 {
		t.Fatal("unscoped legacy token accepted")
	}
	aApp, _ := resolve(ctx, ids[0])
	wrongActor := domain.Principal{FamilyID: ids[1], ParticipantID: 1, Role: domain.RoleParent}
	if _, err := aApp.CreateChore(ctx, wrongActor, domain.Chore{}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal("foreign principal accepted", err)
	}
	r := httptest.NewRequest("GET", "/api/participants", nil)
	r.Header.Set("X-FamilyQuest", "1")
	r.AddCookie(&http.Cookie{Name: "familyquest_device", Value: fmt.Sprint(ids[1]) + "." + strings.Repeat("f", 64)})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("forged cookie accepted", w.Code)
	}
	if w := call("GET", "/api/participants", session.Token, ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	appB, _ := resolve(ctx, ids[1])
	if _, e = appB.ParseToken(ctx, session.Token); e == nil {
		t.Fatal("token crossed family")
	}
	if w := call("GET", "/api/subscription", session.Token, ""); w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	if e = p.RefundPayment(ctx, payment.Reference, "test"); e != nil {
		t.Fatal(e)
	}
	if e = p.RefundPayment(ctx, payment.Reference, "test"); e != nil {
		t.Fatal(e)
	}
	v, e := p.Subscription(ctx, ids[0])
	if e != nil || v.Paid || v.Access {
		t.Fatal("refund did not revoke access", v, e)
	}
	if w := call("POST", "/api/participants", session.Token, `{"name":"Extra","pin":"391827","role":"child"}`); w.Code != 403 {
		t.Fatal("expired write accepted", w.Code)
	}
	if w := call("GET", "/api/participants", session.Token, ""); w.Code != 200 {
		t.Fatal("expired history inaccessible", w.Code)
	}
	if e = p.Suspend(ctx, ids[0], true, "test"); e != nil {
		t.Fatal(e)
	}
	if w := call("GET", "/api/participants", session.Token, ""); w.Code != 401 {
		t.Fatal("suspension bypass", w.Code)
	}
}
