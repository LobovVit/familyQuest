package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVersionIsPublicAndDoesNotReachFamilyHandler(t *testing.T) {
	handler := WithVersion(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }), "https://example.test", VersionInfo{Version: "0.1.0", Commit: "abc", BuiltAt: "2026-09-27T00:00:00Z"})
	for _, tc := range []struct {
		method, path string
		status       int
	}{{"GET", "/api/version", 200}, {"POST", "/api/version", 405}, {"OPTIONS", "/api/version", 204}, {"GET", "/api/participants", 401}} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.status {
			t.Fatalf("%+v: %d", tc, w.Code)
		}
		if tc.status == 200 && (w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Body.String(), `"version":"0.1.0"`)) {
			t.Fatal(w.Result(), w.Body.String())
		}
	}
}
