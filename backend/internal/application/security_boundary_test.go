package application

import (
	"context"
	"errors"
	"github.com/lobov/familyquest/backend/internal/domain"
	"testing"
	"time"
)

type guardedRepository struct {
	Repository
	owner     domain.Participant
	revoked   bool
	mutations int
}

func (r *guardedRepository) GetParticipant(context.Context, int64) (domain.Participant, error) {
	return r.owner, nil
}
func (r *guardedRepository) DeviceAuthorized(context.Context, string, int64, int64) error {
	if r.revoked {
		return domain.ErrUnauthorized
	}
	return nil
}
func (r *guardedRepository) UpdateParticipantPIN(context.Context, int64, string) (domain.Participant, error) {
	r.mutations++
	return r.owner, nil
}
func (r *guardedRepository) ImportBackup(context.Context, BackupData) error {
	r.mutations++
	return nil
}
func TestSensitiveUseCasesRequireProofWithoutHTTP(t *testing.T) {
	actor := domain.Principal{ParticipantID: 1, Role: domain.RoleParent, DeviceID: "ipad", SessionVersion: 3}
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	proof := actor
	proof.ConfirmedUntil = now.Add(time.Minute).Unix()
	backup := BackupData{Version: BackupVersion, Participants: []BackupParticipant{{ID: 1, Name: "Parent", Role: domain.RoleParent, Active: true}}}
	for _, name := range []string{"missing", "expired", "different device", "changed PIN", "revoked device", "valid"} {
		t.Run(name, func(t *testing.T) {
			repo := &guardedRepository{owner: domain.Participant{ID: 1, Role: domain.RoleParent, Active: true, SessionVersion: 3}}
			claim := proof
			raw := "proof"
			switch name {
			case "missing":
				raw = ""
			case "expired":
				claim.ConfirmedUntil = now.Unix()
			case "different device":
				claim.DeviceID = "mac"
			case "changed PIN":
				repo.owner.SessionVersion++
			case "revoked device":
				repo.revoked = true
			}
			service := New(repo, testTokens{claim})
			service.now = func() time.Time { return now }
			_, pinErr := service.UpdateParticipantPIN(context.Background(), actor, 2, "123456", raw)
			backupErr := service.ImportBackup(context.Background(), actor, backup, raw)
			if name == "valid" {
				if pinErr != nil || backupErr != nil || repo.mutations != 2 {
					t.Fatal(pinErr, backupErr, repo.mutations)
				}
			} else {
				if pinErr == nil || backupErr == nil || repo.mutations != 0 {
					t.Fatal("bypassed application boundary", name, pinErr, backupErr)
				}
			}
		})
	}
}

type loginRepository struct {
	Repository
	created   bool
	forgot    bool
	createErr error
	previous  string
}

func (r *loginRepository) VerifyParticipantPIN(_ context.Context, id int64, pin string) (domain.Participant, error) {
	if pin != "123456" {
		return domain.Participant{}, domain.ErrInvalidPIN
	}
	return domain.Participant{ID: id, Role: domain.RoleParent, Active: true}, nil
}
func (r *loginRepository) CreateDevice(_ context.Context, d domain.TrustedDevice, hash string, owner, approver domain.Participant, previous string) error {
	r.created = true
	r.previous = previous
	return r.createErr
}
func (r *loginRepository) RevokeDeviceToken(context.Context, string) error {
	r.forgot = true
	return nil
}
func TestLoginOrchestratesOneAtomicDeviceReplacement(t *testing.T) {
	repo := &loginRepository{createErr: errors.New("rollback")}
	service := New(repo, testTokens{})
	_, err := service.Login(context.Background(), LoginInput{ParticipantID: 1, PIN: "123456", Remember: true, DeviceName: "iPad", PreviousSecret: "old"})
	if err == nil || !repo.created || repo.forgot || repo.previous != deviceHash("old") {
		t.Fatal("replacement is not delegated atomically", err)
	}
	repo.created = false
	_, err = service.Login(context.Background(), LoginInput{ParticipantID: 1, PIN: "000000", Remember: true, DeviceName: "iPad"})
	if err == nil || repo.created || repo.forgot {
		t.Fatal("failed PIN touched device state")
	}
}
