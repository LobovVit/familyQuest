package domain

import "time"

type NotificationRecipient struct {
	Reference      string `json:"reference"`
	SubscriptionID string `json:"subscriptionId"`
	BindingID      string `json:"bindingId"`
	Issuer         string `json:"issuer"`
	Subject        string `json:"subject"`
	ParentID       int64  `json:"parentId"`
}
type NotificationDelivery struct {
	ID        string                `json:"id"`
	Lease     string                `json:"lease"`
	Recipient NotificationRecipient `json:"recipient"`
	Attempts  int                   `json:"attempts"`
}
type NotificationSettlement struct {
	ID        string `json:"id"`
	Lease     string `json:"lease"`
	Outcome   string `json:"outcome"`
	ErrorCode string `json:"errorCode"`
	PortalID  string `json:"portalId"`
}

// Retries never exhaust transient failures. Terminal failures remain in the audit queue.
func NotificationRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 11 {
		return time.Hour
	}
	delay := 5 * time.Second * time.Duration(1<<uint(attempt-1))
	if delay > time.Hour {
		return time.Hour
	}
	return delay
}
