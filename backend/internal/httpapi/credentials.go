package httpapi

import (
	"github.com/lobov/familyquest/backend/internal/domain"
	"strings"
)

// Authorization header syntax belongs to the HTTP adapter, not the token signer.
func parseBearer(header string) (string, error) {
	fields := strings.Fields(header)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") || fields[1] == "" {
		return "", domain.ErrUnauthorized
	}
	return fields[1], nil
}
