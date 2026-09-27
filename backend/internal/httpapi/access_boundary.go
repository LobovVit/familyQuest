package httpapi

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/lobov/familyquest/backend/internal/application"
	"github.com/lobov/familyquest/backend/internal/domain"
)

const AccessKeyHeader = "X-FamilyQuest-Access-Key"
const IntrospectionPath = "/api/__access/introspect"
const AccountSessionPath = "/api/__access/account-session"

type AccessState struct {
	Principal    domain.Principal          `json:"principal"`
	Subscription domain.FamilySubscription `json:"subscription"`
}
type AccountSessionInput struct {
	FamilyID      int64 `json:"familyId"`
	ParticipantID int64 `json:"participantId"`
}

// RequireAccess blocks direct origin access; this key is not a user credential.
// RequireAccess закрывает прямой доступ к приложению; ключ не является пользовательской сессией.
func RequireAccess(next http.Handler, key string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" && r.Method == http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		if len(key) < 32 || subtle.ConstantTimeCompare([]byte(r.Header.Get(AccessKeyHeader)), []byte(key)) != 1 {
			respond(w, nil, domain.ErrUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func NewPrivateSaaS(platform application.PlatformGateway, resolve FamilyServices, tokens application.Tokens, cors string) http.Handler {
	return &SaaS{platform: platform, resolve: resolve, tokens: tokens, cors: cors, attempts: map[string]attempt{}, private: true}
}

// SameSiteRequest preserves the existing CSRF rules at the access boundary.
// SameSiteRequest сохраняет существующие правила CSRF на границе сервиса доступа.
func SameSiteRequest(r *http.Request) bool { return sameSiteRequest(r) }

// AllowsExpiredRequest is shared by the proxy and origin: history remains readable.
// AllowsExpiredRequest общая для прокси и приложения: история остаётся доступной.
func AllowsExpiredRequest(method, route string) bool {
	if method == http.MethodGet || method == http.MethodHead {
		return true
	}
	if method != http.MethodPost {
		return false
	}
	switch route {
	case "/api/session", "/api/session/logout", "/api/session/confirm":
		return true
	}
	parts := strings.Split(route, "/")
	return len(parts) == 5 && parts[1] == "api" && parts[2] == "math" && parts[3] != "" && (parts[4] == "answers" || parts[4] == "finish")
}
