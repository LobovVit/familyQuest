package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	"github.com/lobov/familyquest/backend/internal/testoidc"
)

// Real Access + Core + PostgreSQL; only Portal/Telegram delivery is a test transport.
func TestTelegramMiniAppRealCore(t *testing.T) {
	dbURL := os.Getenv("FAMILYQUEST_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("disposable postgres required")
	}
	t.Chdir("../..")
	ctx := context.Background()
	root, err := Open(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	p := NewPlatform(root)
	if err = p.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	ids := []int64{}
	defer func() {
		for _, id := range ids {
			_, _ = root.pool.Exec(ctx, `delete from fq_platform.audit where family_id=$1`, id)
			_, _ = root.pool.Exec(ctx, `delete from fq_platform.accounts where family_id=$1`, id)
			_, _ = root.pool.Exec(ctx, `delete from fq_platform.families where id=$1`, id)
			_, _ = root.pool.Exec(ctx, "drop schema "+schemaFor(id)+" cascade")
			_, _ = root.pool.Exec(ctx, "drop role "+roleFor(id))
		}
	}()
	for _, name := range []string{"Own", "Foreign"} {
		id, e := p.Provision(ctx, name, fmt.Sprintf("tg-%s-%d@example.test", name, time.Now().UnixNano()), "test-password-long", "Parent", "739281", "test", false)
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
	}
	own, err := p.Family(ctx, ids[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err = root.pool.Exec(ctx, "update "+schemaFor(ids[0])+".service_policy set access_until=null"); err != nil {
		t.Fatal(err)
	}
	child, err := own.CreateParticipant(ctx, domain.Participant{Name: "Private child", Role: domain.RoleChild}, "391827")
	if err != nil {
		t.Fatal(err)
	}
	provider := testoidc.New(t, nil)
	subject := "telegram-parent"
	if err = p.BindIdentity(ctx, provider.Server.URL, subject, ids[0], "test"); err != nil {
		t.Fatal(err)
	}
	_, owner, err := p.Identity(ctx, provider.Server.URL, subject)
	if err != nil {
		t.Fatal(err)
	}
	tokens, _ := auth.New(strings.Repeat("x", 40), time.Hour)
	resolve := func(c context.Context, id int64) (*application.Service, error) {
		repo, e := p.Family(c, id)
		if e != nil {
			return nil, e
		}
		return application.NewForFamily(repo, tokens.ForFamily(id), id), nil
	}
	var mu sync.Mutex
	var failSettleOnce bool
	coreHandler := httpapi.RequireAccess(httpapi.NewPrivateSaaS(CorePlatform{p}, resolve, tokens, ""), strings.Repeat("k", 40))
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		fail := failSettleOnce && r.URL.Path == httpapi.NotificationPath+"settle"
		if fail {
			failSettleOnce = false
		}
		mu.Unlock()
		if fail {
			w.WriteHeader(503)
			return
		}
		coreHandler.ServeHTTP(w, r)
	}))
	defer core.Close()
	g, err := access.New(p, core.URL, strings.Repeat("k", 40), "https://family.example")
	if err != nil {
		t.Fatal(err)
	}
	oidc, err := identity.New(ctx, identity.Config{Issuer: provider.Server.URL, ClientID: "familyquest", ClientSecret: "test-secret", RedirectURL: "https://family.example/api/account/callback"})
	if err != nil {
		t.Fatal(err)
	}
	g.SSO = access.NewSSO(oidc, p)
	linked, revoked, unavailable, consented := false, false, false, false
	events := map[string]map[string]string{}
	exchangeCount := 0
	notificationCalls := 0
	lostNotificationResponse := false
	expires := time.Now().Add(time.Hour)
	portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("p", 40) {
			t.Error("missing Portal product credential")
			w.WriteHeader(401)
			return
		}
		if unavailable {
			w.WriteHeader(503)
			return
		}
		var in map[string]any
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			t.Error("invalid Portal body")
		}
		if _, ok := in["productId"]; ok {
			t.Error("productId must derive from service key")
		}
		w.Header().Set("Content-Type", "application/json")
		principal := map[string]any{"productId": "familyquest", "botId": "familyquest", "recipientRef": "tgr_test_adult", "identityState": "unlinked"}
		if linked {
			principal["identityState"] = "linked"
			principal["issuer"] = provider.Server.URL
			principal["subject"] = subject
		}
		switch r.URL.Path {
		case "/v1/telegram/exchange":
			exchangeCount++
			if exchangeCount > 1 {
				w.WriteHeader(409)
				return
			}
			if in["botId"] != "familyquest" {
				t.Error("wrong bot")
			}
			json.NewEncoder(w).Encode(map[string]any{"sessionToken": "tgs_test_server_only", "expiresAt": expires, "principal": principal})
		case "/v1/telegram/session":
			if revoked || in["sessionToken"] != "tgs_test_server_only" {
				w.WriteHeader(401)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"expiresAt": expires, "principal": principal})
		case "/v1/telegram/link/claim":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		case "/v1/telegram/notifications/consent":
			consented = in["enabled"] == true
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		case "/v1/telegram/notifications":
			if !consented || revoked {
				w.WriteHeader(403)
				return
			}
			if len(in) != 3 || in["template"] != "activity" || in["recipientRef"] != "tgr_test_adult" {
				t.Error("unsafe notification payload")
			}
			key, _ := in["eventKey"].(string)
			events[key] = map[string]string{"template": "activity"}
			notificationCalls++
			if lostNotificationResponse {
				lostNotificationResponse = false
				connection, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				connection.Close()
				return
			}
			w.WriteHeader(202)
			json.NewEncoder(w).Encode(map[string]string{"notificationId": "tgn_test"})
		case "/v1/telegram/logout":
			revoked = true
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		default:
			t.Error("unknown Portal contract path", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer portal.Close()
	transport, err := access.NewTelegramPortal(portal.URL, strings.Repeat("p", 40))
	if err != nil {
		t.Fatal(err)
	}
	g.Telegram = access.NewTelegram(transport, p, provider.Server.URL, "https://portal.example/telegram", true)
	recipientsFile := t.TempDir() + "/recipients.json"
	g.Telegram.Notifications, err = access.NewTelegramNotifications(recipientsFile)
	if err != nil {
		t.Fatal(err)
	}
	var cookie *http.Cookie
	call := func(method, path, body, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://family.example"+path, strings.NewReader(body))
		r.Header.Set("Origin", "https://family.example")
		r.Header.Set("X-FamilyQuest", "1")
		r.Header.Set("X-FamilyQuest-Client", "telegram")
		r.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		g.ServeHTTP(w, r)
		for _, c := range w.Result().Cookies() {
			if c.Name == "familyquest_telegram" {
				cookie = c
			}
		}
		return w
	}
	want := func(w *httptest.ResponseRecorder, code int) {
		t.Helper()
		if w.Code != code {
			t.Fatalf("want %d got %d: %s", code, w.Code, w.Body.String())
		}
	}
	entry := call("POST", "/api/telegram/exchange", `{"initData":"explicit-test-transport-only"}`, "")
	want(entry, 200)
	if strings.Contains(entry.Body.String(), "tgs_") || strings.Contains(entry.Body.String(), "Private child") || cookie == nil || !cookie.HttpOnly || !cookie.Secure {
		t.Fatal("exchange leaks credentials or family data")
	}
	want(call("POST", "/api/telegram/family", fmt.Sprintf(`{"familyId":%d}`, ids[0]), ""), 403)
	want(call("POST", "/api/telegram/profile", fmt.Sprintf(`{"participantId":%d,"pin":"739281"}`, owner), ""), 403)
	// Claim does not link or auto-create anything; confirmation is performed in Portal.
	want(call("POST", "/api/telegram/link", `{"linkCode":"tgl_test_code"}`, ""), 200)
	if linked {
		t.Fatal("claim automatically linked")
	}
	mu.Lock()
	linked = true
	mu.Unlock()
	state := call("POST", "/api/telegram/session", `{}`, "")
	want(state, 200)
	if !strings.Contains(state.Body.String(), "Own") || strings.Contains(state.Body.String(), "Foreign") {
		t.Fatal("family isolation")
	}
	want(call("POST", "/api/telegram/profile", fmt.Sprintf(`{"participantId":%d,"pin":"739281"}`, owner), ""), 403)
	want(call("POST", "/api/telegram/family", fmt.Sprintf(`{"familyId":%d}`, ids[1]), ""), 403)
	want(call("POST", "/api/telegram/family", fmt.Sprintf(`{"familyId":%d}`, ids[0]), ""), 200)
	want(call("POST", "/api/telegram/profile", fmt.Sprintf(`{"participantId":%d,"pin":"000000"}`, owner), ""), 401)
	profile := call("POST", "/api/telegram/profile", fmt.Sprintf(`{"participantId":%d,"pin":"739281"}`, owner), "")
	want(profile, 200)
	var parentSession application.LoginResult
	if err = json.Unmarshal(profile.Body.Bytes(), &parentSession); err != nil {
		t.Fatal(err)
	}
	parentToken := parentSession.Token
	want(call("POST", "/api/telegram/consent", `{"enabled":true}`, ""), 401)
	want(call("POST", "/api/telegram/consent", `{"enabled":true}`, parentToken), 200)
	// Persistence survives restart without any Telegram session token in the file.
	raw, _ := os.ReadFile(recipientsFile)
	if strings.Contains(string(raw), "tgs_") || strings.Contains(string(raw), "Private child") {
		t.Fatal("secret in recipient file")
	}
	g.Telegram.Notifications, err = access.NewTelegramNotifications(recipientsFile)
	if err != nil {
		t.Fatal(err)
	}
	chore := call("POST", "/api/chores", fmt.Sprintf(`{"title":"Read a book","schedule":"daily","baseValue":50,"participantIds":[%d]}`, child.ID), parentToken)
	want(chore, 201)
	tasksResult := call("GET", "/api/tasks?date="+time.Now().Format("2006-01-02"), "", parentToken)
	want(tasksResult, 200)
	var tasks []domain.Task
	if err = json.Unmarshal(tasksResult.Body.Bytes(), &tasks); err != nil || len(tasks) != 1 {
		t.Fatal("tasks", err, tasksResult.Body.String())
	}
	task := tasks[0]
	childEntry := call("POST", "/api/telegram/profile", fmt.Sprintf(`{"participantId":%d,"pin":"391827"}`, child.ID), "")
	want(childEntry, 200)
	var childSession application.LoginResult
	json.Unmarshal(childEntry.Body.Bytes(), &childSession)
	want(call("POST", "/api/telegram/consent", `{"enabled":true}`, childSession.Token), 403)
	want(call("POST", "/api/chores", fmt.Sprintf(`{"title":"Forged","schedule":"daily","baseValue":50,"participantIds":[%d]}`, child.ID), childSession.Token), 403)
	confirmPath := fmt.Sprintf("/api/tasks/%d/confirm", task.ID)
	want(call("POST", confirmPath, `{"rating":5}`, childSession.Token), 403)
	completePath := fmt.Sprintf("/api/tasks/%d/complete", task.ID)
	// Failure to insert the event must roll back the task, not silently lose notification work.
	if _, err = root.pool.Exec(ctx, "alter table "+schemaFor(ids[0])+".notification_outbox add constraint injected_failure check(false) not valid"); err != nil {
		t.Fatal(err)
	}
	want(call("POST", completePath, `{}`, childSession.Token), 500)
	var taskStatus string
	if err = own.db.QueryRow(ctx, `select status from tasks where id=$1`, task.ID).Scan(&taskStatus); err != nil || taskStatus != "pending" {
		t.Fatal("task committed without event", taskStatus, err)
	}
	if _, err = root.pool.Exec(ctx, "alter table "+schemaFor(ids[0])+".notification_outbox drop constraint injected_failure"); err != nil {
		t.Fatal(err)
	}
	want(call("POST", completePath, `{}`, childSession.Token), 200)
	want(call("POST", httpapi.NotificationPath+"claim", fmt.Sprintf(`{"familyId":%d}`, ids[0]), parentToken), 404)
	mu.Lock()
	earlyMessages := len(events)
	unavailable = true
	mu.Unlock()
	if earlyMessages != 0 {
		t.Fatal("request performed synchronous delivery")
	}
	if n, e := g.DispatchTelegramNotifications(ctx, ids[0], 10); e != nil || n != 1 {
		t.Fatal("Portal outage was not queued for retry", n, e)
	}
	var deliveryState, eventID string
	var attempts int
	var deferred bool
	if err = own.db.QueryRow(ctx, `select id::text,state,attempts,available_at>now() from notification_outbox`).Scan(&eventID, &deliveryState, &attempts, &deferred); err != nil || deliveryState != "pending" || attempts != 1 || !deferred {
		t.Fatal("retry state", deliveryState, attempts, deferred, err)
	}
	if n, e := g.DispatchTelegramNotifications(ctx, ids[0], 1); e != nil || n != 0 {
		t.Fatal("ignored backoff", n, e)
	}
	forceReady := func() {
		t.Helper()
		if _, e := own.db.Exec(ctx, `update notification_outbox set available_at=now()-interval '1 second',lease_until=case when lease_token is null then null else now()-interval '1 second' end where state='pending'`); e != nil {
			t.Fatal(e)
		}
	}
	forceReady()
	// Concurrent claimers get one lease. Crash before send is recoverable; stale ack is rejected.
	results := make(chan *domain.NotificationDelivery, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() { job, e := own.ClaimNotification(ctx); results <- job; errs <- e }()
	}
	var crashed *domain.NotificationDelivery
	for i := 0; i < 8; i++ {
		if e := <-errs; e != nil {
			t.Fatal(e)
		}
		if job := <-results; job != nil {
			if crashed != nil {
				t.Fatal("duplicate live lease")
			}
			crashed = job
		}
	}
	if crashed == nil {
		t.Fatal("no claimed event")
	}
	forceReady()
	next, e := own.ClaimNotification(ctx)
	if e != nil || next == nil || next.Lease == crashed.Lease || next.ID != eventID {
		t.Fatal("lease recovery", next, e)
	}
	if e = own.SettleNotification(ctx, domain.NotificationSettlement{ID: crashed.ID, Lease: crashed.Lease, Outcome: "delivered", PortalID: "tgn_stale"}); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("stale worker ack accepted", e)
	}
	forceReady()
	// Simulate process restart: no in-memory job list is carried to the new worker.
	restarted, e := access.New(p, core.URL, strings.Repeat("k", 40), "https://family.example")
	if e != nil {
		t.Fatal(e)
	}
	restarted.Telegram = access.NewTelegram(transport, p, provider.Server.URL, "https://portal.example/telegram", true)
	restarted.Telegram.Notifications, e = access.NewTelegramNotifications(recipientsFile)
	if e != nil {
		t.Fatal(e)
	}
	mu.Lock()
	unavailable = false
	lostNotificationResponse = true
	mu.Unlock()
	if n, e := restarted.DispatchTelegramNotifications(ctx, ids[0], 10); e != nil || n != 1 {
		t.Fatal("lost response retry", n, e)
	}
	if err = own.db.QueryRow(ctx, `select state from notification_outbox where id=$1`, eventID).Scan(&deliveryState); err != nil || deliveryState != "pending" {
		t.Fatal("ambiguous Portal response lost", deliveryState, err)
	}
	forceReady()
	mu.Lock()
	failSettleOnce = true
	mu.Unlock()
	if _, e = restarted.DispatchTelegramNotifications(ctx, ids[0], 1); e == nil {
		t.Fatal("ack failure was hidden")
	}
	if err = own.db.QueryRow(ctx, `select state from notification_outbox where id=$1`, eventID).Scan(&deliveryState); err != nil || deliveryState != "pending" {
		t.Fatal("ack failure dropped event", deliveryState, err)
	}
	forceReady()
	if n, e := restarted.DispatchTelegramNotifications(ctx, ids[0], 10); e != nil || n != 1 {
		t.Fatal("ack recovery", n, e)
	}
	if err = own.db.QueryRow(ctx, `select state,attempts from notification_outbox where id=$1`, eventID).Scan(&deliveryState, &attempts); err != nil || deliveryState != "delivered" || attempts != 6 {
		t.Fatal("delivery audit", deliveryState, attempts, err)
	}
	mu.Lock()
	calls := notificationCalls
	mu.Unlock()
	if calls != 3 {
		t.Fatal("expected three idempotent transport attempts", calls)
	}

	mu.Lock()
	count := len(events)
	mu.Unlock()
	if count != 1 {
		t.Fatal("expected one neutral notification", count)
	}
	want(call("POST", completePath, `{}`, childSession.Token), 404)
	confirmed := call("POST", confirmPath, `{"rating":5}`, parentToken)
	want(confirmed, 200)
	var first domain.Task
	json.Unmarshal(confirmed.Body.Bytes(), &first)
	again := call("POST", confirmPath, `{"rating":5}`, parentToken)
	want(again, 200)
	var second domain.Task
	json.Unmarshal(again.Body.Bytes(), &second)
	if first.Reward != 50 || second.Reward != first.Reward {
		t.Fatal("reward repeated", first.Reward, second.Reward)
	}
	want(call("PUT", "/api/family/name", `{"name":"Renamed family"}`, parentToken), 200)
	// Foreign family bearer cannot be paired with this Telegram session.
	foreignApp, err := resolve(ctx, ids[1])
	if err != nil {
		t.Fatal(err)
	}
	foreignRepo, _ := p.Family(ctx, ids[1])
	people, _ := foreignRepo.ListParticipants(ctx)
	foreignSession, err := foreignApp.AccountSession(ctx, people[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	want(call("GET", "/api/participants", "", foreignSession.Token), 403)
	oldCookie := cookie
	want(call("POST", "/api/telegram/family", `{"familyId":0}`, parentToken), 200)
	want(call("GET", "/api/participants", "", parentToken), 403)
	newCookie := cookie
	cookie = oldCookie
	want(call("GET", "/api/participants", "", parentToken), 401)
	cookie = newCookie
	want(call("POST", "/api/telegram/family", fmt.Sprintf(`{"familyId":%d}`, ids[0]), ""), 200)
	// Pending work is cancelled on opt-out and cannot revive after a new consent generation.
	tomorrowTasks, e := own.ListTasks(ctx, time.Now().Add(24*time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	var tomorrow int64
	for _, v := range tomorrowTasks {
		if v.ID != task.ID {
			tomorrow = v.ID
			break
		}
	}
	if tomorrow == 0 {
		t.Fatal("missing next task")
	}
	if _, e = own.CompleteTask(ctx, tomorrow, child.ID); e != nil {
		t.Fatal(e)
	}
	want(call("POST", "/api/telegram/consent", `{"enabled":false}`, parentToken), 200)
	want(call("POST", "/api/telegram/consent", `{"enabled":true}`, parentToken), 200)
	if n, e := g.DispatchTelegramNotifications(ctx, ids[0], 10); e != nil || n != 0 {
		t.Fatal("old consent event revived", n, e)
	}
	var cancelled int
	if err = own.db.QueryRow(ctx, `select count(*) from notification_outbox where state='cancelled'`).Scan(&cancelled); err != nil || cancelled != 1 {
		t.Fatal("pending optout", cancelled, err)
	}
	// Existing local rights and Portal both remain authoritative.
	_, _ = root.pool.Exec(ctx, "update "+schemaFor(ids[0])+".service_policy set access_until=now()-interval '1 hour'")
	want(call("POST", "/api/chores", fmt.Sprintf(`{"title":"Denied","schedule":"daily","baseValue":50,"participantIds":[%d]}`, child.ID), parentToken), 403)
	want(call("GET", "/api/tasks?date="+time.Now().Format("2006-01-02"), "", parentToken), 200)
	mu.Lock()
	unavailable = true
	mu.Unlock()
	want(call("GET", "/api/participants", "", parentToken), 503)
	mu.Lock()
	unavailable = false
	mu.Unlock()
	want(call("POST", "/api/telegram/consent", `{"enabled":false}`, childSession.Token), 200)
	mu.Lock()
	if consented {
		t.Error("opt out failed")
	}
	revoked = true
	mu.Unlock()
	want(call("GET", "/api/participants", "", parentToken), 401)
	want(call("POST", confirmPath, `{"rating":5}`, parentToken), 401)
	want(call("POST", "/api/telegram/logout", `{}`, ""), 200)
	// Queue scope, current role and backup restore remain authoritative independently of Portal.
	recipient := domain.NotificationRecipient{Reference: "tgr_restore_test", SubscriptionID: "00000000-0000-4000-8000-000000000001", Issuer: provider.Server.URL, Subject: subject, ParentID: owner}
	if err = own.SetNotificationRecipient(ctx, recipient, true); err != nil {
		t.Fatal(err)
	}
	if _, e := root.pool.Exec(ctx, "update "+schemaFor(ids[0])+".service_policy set access_until=null"); e != nil {
		t.Fatal(e)
	}
	laterTasks, e := own.ListTasks(ctx, time.Now().Add(48*time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	var later int64
	for _, v := range laterTasks {
		if v.ID != task.ID && v.ID != tomorrow {
			later = v.ID
			break
		}
	}
	if later == 0 {
		t.Fatal("missing later task")
	}
	if _, e = own.CompleteTask(ctx, later, child.ID); e != nil {
		t.Fatal(e)
	}
	leased, e := own.ClaimNotification(ctx)
	if e != nil || leased == nil {
		t.Fatal("claim before restore", e)
	}
	if foreignJob, e := foreignRepo.ClaimNotification(ctx); e != nil || foreignJob != nil {
		t.Fatal("foreign family saw outbox", foreignJob, e)
	}
	if e = foreignRepo.SettleNotification(ctx, domain.NotificationSettlement{ID: leased.ID, Lease: leased.Lease, Outcome: "delivered", PortalID: "tgn_foreign"}); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("foreign family settled outbox", e)
	}
	if ok, e := own.NotificationAllowed(ctx, leased.ID, leased.Lease); e != nil || !ok {
		t.Fatal("authorized recipient denied", e)
	}
	if _, e = own.db.Exec(ctx, `update participants set active=false where id=$1`, owner); e != nil {
		t.Fatal(e)
	}
	if ok, e := own.NotificationAllowed(ctx, leased.ID, leased.Lease); e != nil || ok {
		t.Fatal("inactive parent retained delivery", e)
	}
	if _, e = own.db.Exec(ctx, `update participants set active=true where id=$1`, owner); e != nil {
		t.Fatal(e)
	}
	snapshot, e := own.ExportBackup(ctx)
	if e != nil {
		t.Fatal(e)
	}
	encoded, _ := json.Marshal(snapshot)
	if strings.Contains(string(encoded), "tgr_restore_test") || strings.Contains(string(encoded), "subscriptionId") {
		t.Fatal("integration metadata leaked to family export")
	}
	if e = own.ImportBackup(ctx, snapshot); e != nil {
		t.Fatal(e)
	}
	if ok, e := own.NotificationAllowed(ctx, leased.ID, leased.Lease); e != nil || ok {
		t.Fatal("restore retained pending lease", e)
	}
	var recipients, delivered int
	if e = own.db.QueryRow(ctx, `select (select count(*) from notification_recipients),(select count(*) from notification_outbox where state='delivered')`).Scan(&recipients, &delivered); e != nil || recipients != 0 || delivered != 1 {
		t.Fatal("restore lost audit or revived subscriptions", recipients, delivered, e)
	}

}
