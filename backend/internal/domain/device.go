package domain

import "time"

const DeviceLifetime = 90 * 24 * time.Hour

type TrustedDevice struct {
	ID              string    `json:"id"`
	ParticipantID   int64     `json:"participantId"`
	ParticipantName string    `json:"participantName"`
	Name            string    `json:"name"`
	CreatedAt       time.Time `json:"createdAt"`
	LastSeenAt      time.Time `json:"lastSeenAt"`
	ExpiresAt       time.Time `json:"expiresAt"`
	Current         bool      `json:"current"`
}
