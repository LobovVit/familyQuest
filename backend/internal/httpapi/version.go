package httpapi

import "net/http"

// VersionInfo describes the running binary, not the current repository checkout.
// VersionInfo описывает работающий бинарник, а не текущую ветку репозитория.
type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuiltAt   string `json:"builtAt"`
	StartedAt string `json:"startedAt"`
}

// WithVersion makes build information public in both domestic and SaaS modes.
// WithVersion публикует версию в домашнем режиме и SaaS, без данных семей.
func WithVersion(next http.Handler, cors string, info VersionInfo) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/version" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Access-Control-Allow-Origin", cors)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-FamilyQuest")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET, OPTIONS")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, info)
	})
}
