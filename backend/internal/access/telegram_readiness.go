package access

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
)

type TelegramTransportState struct {
	Mode  string `json:"mode"`
	Ready bool   `json:"ready"`
}

type TelegramQueueState struct {
	Pending              int64  `json:"pending"`
	Sending              int64  `json:"sending"`
	Uncertain            int64  `json:"uncertain"`
	Failed               int64  `json:"failed"`
	RecentFailed         *int64 `json:"recentFailed,omitempty"`
	OldestDueSeconds     int64  `json:"oldestDueSeconds"`
	OldestSendingSeconds int64  `json:"oldestSendingSeconds"`
}

// TelegramReadiness describes only the scoped Portal dependency, not product acceptance.
type TelegramReadiness struct {
	Scope      string                 `json:"scope"`
	ProductID  string                 `json:"productId"`
	Configured bool                   `json:"configured"`
	State      string                 `json:"state"`
	Ready      bool                   `json:"ready"`
	Reasons    []string               `json:"reasons"`
	Incoming   TelegramTransportState `json:"incoming"`
	Delivery   TelegramTransportState `json:"delivery"`
	Queue      TelegramQueueState     `json:"queue"`
}

// ReadTelegramReadiness never exchanges initData, enables a flag or enqueues a message.
func ReadTelegramReadiness(ctx context.Context, origin, key string) (TelegramReadiness, error) {
	var state TelegramReadiness
	portal, err := NewTelegramPortal(origin, key)
	if err != nil {
		return state, errors.New("portal_configuration_missing_or_invalid")
	}
	p := portal.(*telegramHTTP)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.origin+"/v1/telegram/readiness", nil)
	if err != nil {
		return state, errors.New("portal_configuration_missing_or_invalid")
	}
	req.Header.Set("Authorization", "Bearer "+p.key)
	res, err := p.client.Do(req)
	if err != nil {
		return state, errors.New("portal_unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return state, fmt.Errorf("portal_http_%d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 65537))
	if err != nil || len(body) > 65536 {
		return state, errors.New("portal_response_invalid")
	}
	return decodeTelegramReadiness(body)
}

var readinessReason = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func decodeTelegramReadiness(body []byte) (TelegramReadiness, error) {
	var state TelegramReadiness
	invalid := errors.New("portal_response_invalid")
	// Required false/zero values must not be confused with missing or null fields.
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil || json.Unmarshal(body, &state) != nil {
		return TelegramReadiness{}, invalid
	}
	required := func(m map[string]json.RawMessage, keys ...string) bool {
		for _, k := range keys {
			if len(m[k]) == 0 || string(m[k]) == "null" {
				return false
			}
		}
		return true
	}
	if !required(fields, "scope", "productId", "configured", "state", "ready", "reasons", "incoming", "delivery", "queue") || state.Scope != "portal_telegram" || state.ProductID != "familyquest" {
		return TelegramReadiness{}, invalid
	}
	switch state.State {
	case "disabled", "unconfigured", "pilot", "blocked", "ready":
	default:
		return TelegramReadiness{}, invalid
	}
	for _, k := range []string{"incoming", "delivery"} {
		var nested map[string]json.RawMessage
		if json.Unmarshal(fields[k], &nested) != nil || !required(nested, "mode", "ready") {
			return TelegramReadiness{}, invalid
		}
	}
	var queue map[string]json.RawMessage
	if json.Unmarshal(fields["queue"], &queue) != nil || !required(queue, "pending", "sending", "uncertain", "failed", "oldestDueSeconds", "oldestSendingSeconds") {
		return TelegramReadiness{}, invalid
	}
	for _, n := range []int64{state.Queue.Pending, state.Queue.Sending, state.Queue.Uncertain, state.Queue.Failed, state.Queue.OldestDueSeconds, state.Queue.OldestSendingSeconds} {
		if n < 0 {
			return TelegramReadiness{}, invalid
		}
	}
	if len(state.Reasons) > 32 || !readinessReason.MatchString(state.Incoming.Mode) || !readinessReason.MatchString(state.Delivery.Mode) {
		return TelegramReadiness{}, invalid
	}
	for _, reason := range state.Reasons {
		if !readinessReason.MatchString(reason) {
			return TelegramReadiness{}, invalid
		}
	}
	// An inconsistent or future transport mode cannot turn a false dependency green.
	failed := state.Queue.Failed
	if state.Queue.RecentFailed != nil {
		failed = *state.Queue.RecentFailed
		if failed < 0 || failed > state.Queue.Failed {
			return TelegramReadiness{}, invalid
		}
	}
	if state.Ready != (state.State == "ready") || state.Ready && (!state.Configured || len(state.Reasons) != 0 || !state.Incoming.Ready || !state.Delivery.Ready || (state.Incoming.Mode != "polling" && state.Incoming.Mode != "webhook") || state.Delivery.Mode != "linked" || state.Queue.Uncertain != 0 || failed != 0 || state.Queue.OldestDueSeconds > 300 || state.Queue.OldestSendingSeconds > 30) {
		return TelegramReadiness{}, invalid
	}
	return state, nil
}
