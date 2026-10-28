package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/lobov/familyquest/backend/internal/domain"
)

type MCPRepository interface {
	CreateMCPGrant(context.Context, domain.MCPGrant, int64) error
	MCPGrant(context.Context, string, int64, int64, time.Time) (domain.MCPGrant, error)
	ListMCPGrants(context.Context) ([]domain.MCPGrant, error)
	RevokeMCPGrant(context.Context, string) error
}
type mcpIssuer interface {
	IssueMCP(domain.Participant, string, time.Time) (string, error)
}

func (s *Service) CreateMCPGrant(ctx context.Context, p domain.Principal, target int64, name string, readOnly bool, proof string) (domain.MCPGrant, string, error) {
	var g domain.MCPGrant
	if p.MCPGrantID != "" {
		return g, "", domain.ErrForbidden
	}
	if err := s.CheckConfirmation(ctx, p, proof); err != nil {
		return g, "", err
	}
	if err := s.writable(ctx, p, interactiveMutation); err != nil {
		return g, "", err
	}
	name = strings.TrimSpace(name)
	if len([]rune(name)) < 1 || len([]rune(name)) > 80 {
		return g, "", domain.ErrInvalidInput
	}
	owner, err := s.repo.GetParticipant(ctx, target)
	if err != nil {
		return g, "", err
	}
	if !owner.Active {
		return g, "", domain.ErrForbidden
	}
	if owner.Role == domain.RoleSchool {
		readOnly = true
	}
	issuer, ok := s.tokens.(mcpIssuer)
	if !ok {
		return g, "", domain.ErrForbidden
	}
	var id [32]byte
	if _, err = rand.Read(id[:]); err != nil {
		return g, "", err
	}
	g = domain.MCPGrant{ID: hex.EncodeToString(id[:]), ParticipantID: target, Name: name, ReadOnly: readOnly, CreatedAt: s.now(), ExpiresAt: s.now().Add(90 * 24 * time.Hour)}
	token, err := issuer.IssueMCP(owner, g.ID, g.ExpiresAt)
	if err != nil {
		return g, "", err
	}
	err = s.repo.CreateMCPGrant(ctx, g, owner.SessionVersion)
	if err != nil {
		return domain.MCPGrant{}, "", err
	}
	return g, token, nil
}
func (s *Service) MCPGrants(ctx context.Context, p domain.Principal) ([]domain.MCPGrant, error) {
	if !p.IsParent() || p.MCPGrantID != "" || (s.familyID > 0 && p.FamilyID != s.familyID) {
		return nil, domain.ErrForbidden
	}
	return s.repo.ListMCPGrants(ctx)
}
func (s *Service) RemoveMCPGrant(ctx context.Context, p domain.Principal, id string) error {
	if !p.IsParent() || p.MCPGrantID != "" || (s.familyID > 0 && p.FamilyID != s.familyID) {
		return domain.ErrForbidden
	}
	return s.repo.RevokeMCPGrant(ctx, id)
}
