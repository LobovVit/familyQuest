package httpapi

import (
	"net/http"
	"strings"

	"github.com/lobov/familyquest/backend/internal/application"
	"github.com/lobov/familyquest/backend/internal/domain"
)

// Authorization header syntax belongs to the HTTP adapter, not the token signer.
func parseBearer(header string) (string, error) {
	fields := strings.Fields(header)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") || fields[1] == "" {
		return "", domain.ErrUnauthorized
	}
	return fields[1], nil
}

func IsHelperRequest(r *http.Request) bool {
	token, err := parseBearer(r.Header.Get("Authorization"))
	return err == nil && strings.HasPrefix(token, application.HelperTokenPrefix)
}
