package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/lobov/familyquest/backend/internal/domain"
)

const HelperTokenPrefix = "fq-helper."

type HelperAccess struct {
	ID        int64     `json:"id"`
	Label     string    `json:"label"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type HelperTask struct {
	Title string `json:"title"`
	Date  string `json:"date"`
}

type HelperRepository interface {
	CreateHelperAccess(context.Context, []byte, string, []int64) (HelperAccess, error)
	ListHelperAccess(context.Context) ([]HelperAccess, error)
	RevokeHelperAccess(context.Context, int64) error
	HelperTasks(context.Context, []byte) ([]HelperTask, error)
}

func (s *Service) authorizeHelperAdmin(p domain.Principal) error {
	if s.familyID <= 0 || !p.IsParent() {
		return domain.ErrForbidden
	}
	return s.authorizeMutation(p, interactiveMutation)
}

func (s *Service) CreateHelperAccess(ctx context.Context, p domain.Principal, label string, taskIDs []int64) (HelperAccess, string, error) {
	if err := s.authorizeHelperAdmin(p); err != nil {
		return HelperAccess{}, "", err
	}
	if err := s.writable(ctx, p, interactiveMutation); err != nil {
		return HelperAccess{}, "", err
	}
	label = strings.TrimSpace(label)
	if label == "" || len(label) > 160 || len(taskIDs) == 0 || len(taskIDs) > 30 {
		return HelperAccess{}, "", domain.ErrInvalidInput
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return HelperAccess{}, "", err
	}
	hash := sha256.Sum256(secret)
	access, err := s.repo.CreateHelperAccess(ctx, hash[:], label, taskIDs)
	if err != nil {
		return HelperAccess{}, "", err
	}
	return access, HelperTokenPrefix + strconv.FormatInt(s.familyID, 10) + "." + base64.RawURLEncoding.EncodeToString(secret), nil
}

func (s *Service) ListHelperAccess(ctx context.Context, p domain.Principal) ([]HelperAccess, error) {
	if err := s.authorizeHelperAdmin(p); err != nil {
		return nil, err
	}
	return s.repo.ListHelperAccess(ctx)
}

func (s *Service) RevokeHelperAccess(ctx context.Context, p domain.Principal, id int64) error {
	if err := s.authorizeHelperAdmin(p); err != nil {
		return err
	}
	return s.repo.RevokeHelperAccess(ctx, id)
}

func ParseHelperToken(token string) (int64, []byte, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 || parts[0]+"." != HelperTokenPrefix {
		return 0, nil, domain.ErrUnauthorized
	}
	family, err := strconv.ParseInt(parts[1], 10, 64)
	secret, decodeErr := base64.RawURLEncoding.Strict().DecodeString(parts[2])
	if err != nil || family <= 0 || decodeErr != nil || len(secret) != 32 {
		return 0, nil, domain.ErrUnauthorized
	}
	hash := sha256.Sum256(secret)
	return family, hash[:], nil
}

func (s *Service) HelperTasks(ctx context.Context, token string) ([]HelperTask, error) {
	family, hash, err := ParseHelperToken(token)
	if err != nil || family != s.familyID {
		return nil, domain.ErrUnauthorized
	}
	return s.repo.HelperTasks(ctx, hash)
}
