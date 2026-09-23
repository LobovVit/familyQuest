package application

import (
	"context"
	"github.com/lobov/familyquest/backend/internal/domain"
)

// LoginInput expresses one sign-in intent; the adapter supplies the previous device secret.
type LoginInput struct {
	ParticipantID  int64  `json:"participantId"`
	PIN            string `json:"pin"`
	Remember       bool   `json:"remember"`
	DeviceName     string `json:"deviceName"`
	ParentID       int64  `json:"parentId"`
	ParentPIN      string `json:"parentPin"`
	PreviousSecret string `json:"-"`
}
type LoginResult struct {
	Participant  domain.Participant `json:"participant"`
	Token        string             `json:"token"`
	Remembered   bool               `json:"remembered"`
	DeviceID     string             `json:"deviceId"`
	DeviceSecret string             `json:"-"`
}

func (s *Service) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	owner, token, err := s.Authenticate(ctx, input.ParticipantID, input.PIN)
	if err != nil {
		return LoginResult{}, err
	}
	result := LoginResult{Participant: owner, Token: token}
	if input.Remember {
		secret, device, err := s.rememberDevice(ctx, owner, input.DeviceName, input.ParentID, input.ParentPIN, input.PreviousSecret)
		if err != nil {
			return LoginResult{}, err
		}
		result.Token = ""
		result.DeviceID = device.ID
		result.DeviceSecret = secret
		result.Remembered = true
	} else if err = s.ForgetDevice(ctx, input.PreviousSecret); err != nil {
		return LoginResult{}, err
	}
	return result, nil
}
