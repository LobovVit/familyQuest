package store

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lobov/familyquest/backend/internal/access"
	"github.com/lobov/familyquest/backend/internal/application"
	"github.com/lobov/familyquest/backend/internal/auth"
	"github.com/lobov/familyquest/backend/internal/httpapi"
	"github.com/lobov/familyquest/backend/internal/identity"
	"github.com/lobov/familyquest/backend/internal/testoidc"
)

// Real Access/Core/Postgres, real HTTPS sockets and real browser cookie policy.
// Only the external Portal/IdP and HTML shell are isolated synthetic fixtures.
func TestTelegramCrossSiteHTTPSBrowser(t *testing.T) {
	if os.Getenv("FAMILYQUEST_BROWSER_TEST") != "1" {
		t.Skip("opt-in browser regression; see docs/telegram-browser-regression.md")
	}
	dbURL := os.Getenv("FAMILYQUEST_TEST_DATABASE_URL")
	u, err := url.Parse(dbURL)
	if err != nil || u.Hostname() != "127.0.0.1" || u.Path != "/familyquest_browser_test" {
		t.Fatal("requires dedicated loopback database named familyquest_browser_test")
	}
	t.Chdir("../..")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root, err := Open(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	p := NewPlatform(root)
	if err = p.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	family, err := p.Provision(ctx, "Browser fixture", fmt.Sprintf("browser-%d@example.test", time.Now().UnixNano()), "synthetic-password-only", "Browser parent", "739281", "browser-test", false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		for _, sql := range []string{
			fmt.Sprintf("delete from fq_platform.audit where family_id=%d", family),
			fmt.Sprintf("delete from fq_platform.accounts where family_id=%d", family),
			fmt.Sprintf("delete from fq_platform.families where id=%d", family),
			"drop schema " + schemaFor(family) + " cascade",
			"drop role " + roleFor(family),
		} {
			if _, e := root.pool.Exec(cleanup, sql); e != nil {
				t.Error("browser fixture cleanup failed", e)
			}
		}
	}()
	if _, err = root.pool.Exec(ctx, "update "+schemaFor(family)+".service_policy set access_until=null"); err != nil {
		t.Fatal(err)
	}
	provider := testoidc.New(t, nil)
	if err = p.BindIdentity(ctx, provider.Server.URL, "shared-person", family, "browser-test"); err != nil {
		t.Fatal(err)
	}
	_, owner, err := p.Identity(ctx, provider.Server.URL, "shared-person")
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := auth.New(strings.Repeat("b", 40), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	resolve := func(c context.Context, id int64) (*application.Service, error) {
		repo, e := p.Family(c, id)
		if e != nil {
			return nil, e
		}
		return application.NewForFamily(repo, tokens.ForFamily(id), id), nil
	}
	coreKey := strings.Repeat("k", 40)
	core := httptest.NewServer(httpapi.RequireAccess(httpapi.NewPrivateSaaS(CorePlatform{p}, resolve, tokens, ""), coreKey))
	defer core.Close()
	web := httptest.NewUnstartedServer(nil)
	defer web.Close()
	_, port, _ := net.SplitHostPort(web.Listener.Addr().String())
	origin := "https://product.test:" + port
	g, err := access.New(p, core.URL, coreKey, origin)
	if err != nil {
		t.Fatal(err)
	}
	// Complete an actual browser redirect + PKCE + signed ID-token flow. The
	// synthetic IdP approves this one fixture account, without a login GUI.
	providerHandler := provider.Server.Config.Handler
	provider.Server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/authorize" {
			providerHandler.ServeHTTP(w, r)
			return
		}
		code, state := provider.Code(t, r.URL.String())
		http.Redirect(w, r, origin+"/api/account/callback?"+url.Values{"code": {code}, "state": {state}}.Encode(), http.StatusFound)
	})
	oidc, err := identity.New(ctx, identity.Config{Issuer: provider.Server.URL, ClientID: "familyquest", ClientSecret: "test-secret", RedirectURL: origin + "/api/account/callback"})
	if err != nil {
		t.Fatal(err)
	}
	g.SSO = access.NewSSO(oidc, p)
	portal := browserPortal(t, provider.Server.URL)
	transport, err := access.NewTelegramPortal(portal.URL, strings.Repeat("p", 40))
	if err != nil {
		t.Fatal(err)
	}
	g.Telegram = access.NewTelegram(transport, p, provider.Server.URL, "https://unused.example.test/telegram", true)
	var stripped atomic.Int32
	web.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		host, _, _ := net.SplitHostPort(r.Host)
		if host == "container.test" || host == "other-container.test" {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<!doctype html><title>Isolated container</title><iframe src="%s/telegram"></iframe>`, origin)
			return
		}
		if host != "product.test" {
			http.Error(w, "unknown fixture host", 400)
			return
		}
		if r.URL.Path == "/" || r.URL.Path == "/telegram" {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<!doctype html><title>Isolated FamilyQuest shell</title>")
			return
		}
		if r.Header.Get("X-Fixture-Without-Partitioned") == "1" {
			// Negative control changes only the real handler's Set-Cookie attribute.
			// No browser route interception, synthetic API response or cookie injection.
			w = &unpartitionedWriter{ResponseWriter: w, stripped: &stripped}
		}
		g.ServeHTTP(w, r)
	})
	web.StartTLS()
	input, _ := json.Marshal(map[string]any{
		"origin": origin, "containerOrigin": "https://container.test:" + port,
		"otherContainerOrigin": "https://other-container.test:" + port,
		"familyId":             family, "participantId": owner,
		"negativeInitData": signedBrowserInitData("negative"), "positiveInitData": signedBrowserInitData("positive"),
	})
	command := exec.CommandContext(ctx, "node", "../scripts/browser/telegram-https.cjs")
	command.Stdin = bytes.NewReader(input)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("isolated browser regression: %v\n%s", err, output)
	}
	if stripped.Load() != 1 {
		t.Fatalf("negative control modified %d cookies, expected one", stripped.Load())
	}
	var report struct {
		Passed bool `json:"passed"`
	}
	if json.Unmarshal(output, &report) != nil || !report.Passed {
		t.Fatal("browser did not provide a passing report")
	}
	if target := os.Getenv("FAMILYQUEST_BROWSER_REPORT"); target != "" {
		if err := os.WriteFile(target, output, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Log(string(output))
}

type unpartitionedWriter struct {
	http.ResponseWriter
	stripped *atomic.Int32
	wrote    bool
}

func (w *unpartitionedWriter) WriteHeader(code int) {
	if !w.wrote {
		w.wrote = true
		cookies := w.Header().Values("Set-Cookie")
		w.Header().Del("Set-Cookie")
		for _, value := range cookies {
			if strings.HasPrefix(value, "familyquest_telegram=") && strings.Contains(value, "; Partitioned") {
				value = strings.ReplaceAll(value, "; Partitioned", "")
				w.stripped.Add(1)
			}
			w.Header().Add("Set-Cookie", value)
		}
	}
	w.ResponseWriter.WriteHeader(code)
}
func (w *unpartitionedWriter) Write(body []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

// Synthetic Portal fixture, not a replacement for production Portal validation.
// Access deliberately delegates Telegram signature/replay validation to Portal.
const browserBotSecret = "isolated-browser-fixture-only-not-a-real-bot-token"

func browserInitHash(v url.Values) string {
	keys := make([]string, 0, len(v))
	for key := range v {
		if key != "hash" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+"="+v.Get(key))
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(browserBotSecret))
	hash := hmac.New(sha256.New, secret.Sum(nil))
	hash.Write([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(hash.Sum(nil))
}
func signedBrowserInitData(query string) string {
	v := url.Values{"query_id": {query}, "auth_date": {strconv.FormatInt(time.Now().Unix(), 10)}, "user": {`{"id":123456,"first_name":"Synthetic"}`}}
	v.Set("hash", browserInitHash(v))
	return v.Encode()
}
func browserPortal(t *testing.T, issuer string) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	used := map[string]bool{}
	sessions := map[string]time.Time{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("p", 40) {
			w.WriteHeader(401)
			return
		}
		var in map[string]string
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			w.WriteHeader(400)
			return
		}
		token := in["sessionToken"]
		switch r.URL.Path {
		case "/v1/telegram/exchange":
			v, err := url.ParseQuery(in["initData"])
			date, dateErr := strconv.ParseInt(v.Get("auth_date"), 10, 64)
			age := time.Since(time.Unix(date, 0))
			if err != nil || dateErr != nil || age < 0 || age > time.Minute || in["botId"] != "familyquest" || !hmac.Equal([]byte(v.Get("hash")), []byte(browserInitHash(v))) {
				w.WriteHeader(401)
				return
			}
			if used[v.Get("hash")] {
				w.WriteHeader(409)
				return
			}
			used[v.Get("hash")] = true
			token = "synthetic-portal-session-" + v.Get("query_id")
			sessions[token] = time.Now().Add(time.Hour)
		case "/v1/telegram/session":
			if !sessions[token].After(time.Now()) {
				w.WriteHeader(401)
				return
			}
		case "/v1/telegram/logout":
			delete(sessions, token)
			fmt.Fprint(w, `{"ok":true}`)
			return
		default:
			t.Error("unexpected external Portal operation", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"sessionToken": token, "expiresAt": sessions[token], "principal": access.TelegramPrincipal{
			ProductID: "familyquest", BotID: "familyquest", IdentityState: "linked", Issuer: issuer, Subject: "shared-person", RecipientRef: "synthetic-browser-recipient",
		}})
	}))
	t.Cleanup(server.Close)
	return server
}
