package domain

import "time"

type LOVITEntitlement struct {
	ProductID      string           `json:"productId"`
	SubscriptionID string           `json:"subscriptionId"`
	SubjectRef     string           `json:"subjectRef"`
	Access         string           `json:"access"`
	Reason         string           `json:"reason"`
	Paid           bool             `json:"paid"`
	ValidUntil     *time.Time       `json:"validUntil"`
	Revision       int64            `json:"revision"`
	Features       map[string]bool  `json:"features"`
	Limits         map[string]int64 `json:"limits"`
	EvaluatedAt    time.Time        `json:"evaluatedAt"`
}

func (e LOVITEntitlement) Validate(subject, subscription string, now time.Time) error {
	if e.ProductID != "familyquest" || e.SubjectRef != subject || e.SubscriptionID != subscription || e.Revision < 0 || e.EvaluatedAt.Before(now.Add(-5*time.Minute)) || e.EvaluatedAt.After(now.Add(30*time.Second)) {
		return ErrInvalidInput
	}
	switch e.Access {
	case "active":
		if !e.Paid || e.ValidUntil == nil || !e.ValidUntil.After(now) || !e.Features["children_unlimited"] {
			return ErrInvalidInput
		}
	case "expired", "suspended":
	default:
		return ErrInvalidInput
	}
	return nil
}

type LOVITBinding struct {
	FamilyID       int64
	SubscriptionID string
}
