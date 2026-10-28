package access

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lobov/familyquest/backend/internal/domain"
	"github.com/lobov/familyquest/backend/internal/httpapi"
)

type fakeCatalog struct{}

func (fakeCatalog) Account(context.Context, string, string) (int64, int64, error) { return 1, 2, nil }
func (fakeCatalog) Paid(context.Context, int64) (bool, error)                     { return true, nil }
func TestGatewayBoundary(t *testing.T) {
	key := strings.Repeat("s", 48)
	var allowed atomic.Bool
	allowed.Store(true)
	var forwarded atomic.Int64
	origin := httptest.NewServer(httpapi.RequireAccess(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "family.example" {
			t.Errorf("original host lost: %s", r.Host)
		}
		if r.URL.Path == httpapi.IntrospectionPath {
			if r.Header.Get("Authorization") != "Bearer valid" {
				w.WriteHeader(401)
				return
			}
			_ = json.NewEncoder(w).Encode(httpapi.AccessState{Principal: domain.Principal{FamilyID: 1, ParticipantID: 2, Role: domain.RoleParent}, Subscription: domain.FamilySubscription{FamilyID: 1, Status: "active", Access: allowed.Load()}})
			return
		}
		forwarded.Add(1)
		if r.Header.Get("X-Family-ID") != "" || r.Header.Get("X-FamilyQuest-Internal-Actor") != "" {
			t.Error("spoofed header forwarded")
		}
		if r.URL.Path == "/api/family/1" {
			body, _ := io.ReadAll(r.Body)
			if string(body) != "preserved-body" || r.URL.RawQuery != "q=1" {
				t.Error("proxy lost body/query")
			}
		}
		w.WriteHeader(200)
	}), key))
	defer origin.Close()
	g, e := New(fakeCatalog{}, origin.URL, key, "")
	if e != nil {
		t.Fatal(e)
	}
	call := func(method, route, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://family.example"+route, strings.NewReader("preserved-body"))
		r.Header.Set("Authorization", token)
		r.Header.Set("X-FamilyQuest", "1")
		r.Header.Set("Origin", "http://family.example")
		r.Header.Set(httpapi.AccessKeyHeader, "forged")
		r.Header.Set("X-Family-ID", "900")
		r.Header.Set("X-FamilyQuest-Internal-Actor", "admin")
		w := httptest.NewRecorder()
		g.ServeHTTP(w, r)
		return w
	}
	for _, route := range []string{"/api/__access/introspect", "/api/__access/account-session", "/api/a/../__access/account-session", "/api/%5f%5faccess/introspect"} {
		if w := call("GET", route, "Bearer valid"); w.Code != 404 {
			t.Fatal(route, w.Code)
		}
	}
	if w := call("POST", "/api/family/1?q=1", "Bearer valid"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if forwarded.Load() != 1 {
		t.Fatal("unexpected forwarding", forwarded.Load())
	}
	if w := call("GET", "/api/participants", "Bearer invalid"); w.Code != 401 {
		t.Fatal(w.Code)
	}
	allowed.Store(false)
	if w := call("POST", "/api/family/1?q=1", "Bearer valid"); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if forwarded.Load() != 1 {
		t.Fatal("expired write forwarded")
	}
	if w := call("GET", "/api/participants", "Bearer valid"); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := call("POST", "/api/math/session/answers", "Bearer valid"); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := call("POST", "/api/family/answers", "Bearer valid"); w.Code != 403 {
		t.Fatal("loose suffix exception", w.Code)
	}
	origin.Close()
	if w := call("GET", "/api/participants", "Bearer valid"); w.Code != 503 {
		t.Fatal("unavailable introspection allowed", w.Code)
	}
}
func TestOriginRejectsDirectAccess(t *testing.T) {
	h := httpapi.RequireAccess(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }), strings.Repeat("a", 40))
	for _, route := range []string{"/api/participants", "/api/version", httpapi.AccountSessionPath} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", route, nil))
		if w.Code != 401 {
			t.Fatal(route, w.Code)
		}
	}
}

func TestVersionAggregationAndCSRF(t *testing.T) {
	key := strings.Repeat("v", 40)
	origin := httptest.NewServer(httpapi.RequireAccess(httpapi.WithVersion(http.NotFoundHandler(), "", httpapi.VersionInfo{Version: "core", Commit: "core-sha"}), key))
	defer origin.Close()
	g, err := New(fakeCatalog{}, origin.URL, key, "")
	if err != nil {
		t.Fatal(err)
	}
	g.Version = &httpapi.VersionInfo{Version: "access", Commit: "access-sha"}
	w := httptest.NewRecorder()
	g.ServeHTTP(w, httptest.NewRequest("GET", "/api/version", nil))
	var v struct {
		httpapi.VersionInfo
		Access httpapi.VersionInfo `json:"access"`
	}
	if json.Unmarshal(w.Body.Bytes(), &v) != nil || w.Code != 200 || v.Commit != "core-sha" || v.Access.Commit != "access-sha" {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, route := range []string{"/api/account/login", "/api/session/logout", "/api/participants"} {
		r := httptest.NewRequest("POST", "https://family.example"+route, strings.NewReader(`{"email":"owner@example.test","password":"test"}`))
		r.Header.Set("X-FamilyQuest", "1")
		r.Header.Set("Origin", "https://attacker.example")
		w := httptest.NewRecorder()
		g.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal(route, w.Code)
		}
	}
}
