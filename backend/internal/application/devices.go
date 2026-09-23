package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/lobov/familyquest/backend/internal/domain"
)

type DeviceRepository interface {
	CreateDevice(context.Context, domain.TrustedDevice, string, domain.Participant, domain.Participant) error
	AuthenticateDevice(context.Context, string) (domain.Participant, domain.TrustedDevice, error)
	ListDevices(context.Context) ([]domain.TrustedDevice, error)
	RevokeDevice(context.Context, string) error
	RevokeDeviceToken(context.Context, string) error
}

func deviceHash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
func (s *Service) RememberDevice(ctx context.Context, owner domain.Participant, name string, parentID int64, parentPIN string) (string, domain.TrustedDevice, error) {
	name = strings.TrimSpace(name)
	if !domain.IsFamilyMember(domain.Principal{ParticipantID: owner.ID, Role: owner.Role}) || len([]rune(name)) < 1 || len([]rune(name)) > 80 {
		return "", domain.TrustedDevice{}, domain.ErrInvalidInput
	}
	var approver domain.Participant
	if owner.Role == domain.RoleChild {
		if domain.ValidatePIN(parentPIN) != nil {
			return "", domain.TrustedDevice{}, domain.ErrInvalidPIN
		}
		var err error
		approver, err = s.repo.VerifyParticipantPIN(ctx, parentID, parentPIN)
		if err != nil {
			return "", domain.TrustedDevice{}, err
		}
		if approver.Role != domain.RoleParent {
			return "", domain.TrustedDevice{}, domain.ErrForbidden
		}
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", domain.TrustedDevice{}, err
	}
	secret := hex.EncodeToString(raw)
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return "", domain.TrustedDevice{}, err
	}
	d := domain.TrustedDevice{ID: hex.EncodeToString(id), ParticipantID: owner.ID, Name: name, CreatedAt: time.Now().UTC(), LastSeenAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(domain.DeviceLifetime), Current: true}
	err := s.repo.CreateDevice(ctx, d, deviceHash(secret), owner, approver)
	return secret, d, err
}
func (s *Service) DeviceSession(ctx context.Context, secret string) (domain.Participant, domain.TrustedDevice, error) {
	if len(secret) != 64 {
		return domain.Participant{}, domain.TrustedDevice{}, domain.ErrUnauthorized
	}
	return s.repo.AuthenticateDevice(ctx, deviceHash(secret))
}
func (s *Service) ForgetDevice(ctx context.Context, secret string) error {
	if len(secret) != 64 {
		return nil
	}
	return s.repo.RevokeDeviceToken(ctx, deviceHash(secret))
}
func (s *Service) Devices(ctx context.Context, p domain.Principal) ([]domain.TrustedDevice, error) {
	if !p.IsParent() {
		return nil, domain.ErrForbidden
	}
	ds, err := s.repo.ListDevices(ctx)
	for i := range ds {
		ds[i].Current = ds[i].ID == p.DeviceID
	}
	return ds, err
}
func (s *Service) ConfirmParent(ctx context.Context, p domain.Principal, pin string) (string, error) {
	if !p.IsParent() {
		return "", domain.ErrForbidden
	}
	if err := domain.ValidatePIN(pin); err != nil {
		return "", err
	}
	owner, err := s.repo.VerifyParticipantPIN(ctx, p.ParticipantID, pin)
	if err != nil {
		return "", err
	}
	if owner.SessionVersion != p.SessionVersion || owner.Role != p.Role {
		return "", domain.ErrUnauthorized
	}
	return s.tokens.IssueConfirmation(p)
}
func (s *Service) CheckConfirmation(p domain.Principal, proof string) error {
	v, err := s.tokens.Parse(proof)
	if err != nil || !p.IsParent() || v.ConfirmedUntil <= time.Now().Unix() || v.ParticipantID != p.ParticipantID || v.SessionVersion != p.SessionVersion || v.Role != p.Role || v.DeviceID != p.DeviceID {
		return domain.ErrForbidden
	}
	return nil
}
func (s *Service) RemoveDevice(ctx context.Context, p domain.Principal, id, proof string) error {
	if err := s.CheckConfirmation(p, proof); err != nil {
		return err
	}
	return s.repo.RevokeDevice(ctx, id)
}
