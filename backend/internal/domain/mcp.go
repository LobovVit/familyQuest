package domain

import "time"

// MCPGrant is a revocable delegation to a single profile, never a browser session.
type MCPGrant struct {
	ID            string    `json:"id"`
	ParticipantID int64     `json:"participantId"`
	Name          string    `json:"name"`
	ReadOnly      bool      `json:"readOnly"`
	CreatedAt     time.Time `json:"createdAt"`
	ExpiresAt     time.Time `json:"expiresAt"`
}
