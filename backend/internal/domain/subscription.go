package domain

import "time"

type FamilySubscription struct {
	FamilyID       int64      `json:"familyId"`
	Name           string     `json:"name"`
	RegisteredAt   time.Time  `json:"registeredAt"`
	Status         string     `json:"status"`
	Plan           string     `json:"plan"`
	ChildLimit     int        `json:"childLimit"`
	ActiveChildren int        `json:"activeChildren"`
	AccessUntil    *time.Time `json:"accessUntil"`
	Paid           bool       `json:"paid"`
	Access         bool       `json:"access"`
	OwnerEmail     string     `json:"ownerEmail,omitempty"`
}
