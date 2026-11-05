package access

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const TelegramOutboxHeartbeatPath = "/tmp/familyquest-telegram-outbox.json"

// This ephemeral process signal contains no recipients, sessions or delivery authority.
type TelegramOutboxHeartbeat struct {
	CheckedAt time.Time `json:"checkedAt"`
	State     string    `json:"state"`
	ErrorCode string    `json:"errorCode,omitempty"`
}

func WriteTelegramOutboxHeartbeat(path string, value TelegramOutboxHeartbeat) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".outbox-heartbeat-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}

func ReadTelegramOutboxHeartbeat(path string, now time.Time) (TelegramOutboxHeartbeat, error) {
	var value TelegramOutboxHeartbeat
	data, err := os.ReadFile(path)
	if err != nil || len(data) > 1024 || json.Unmarshal(data, &value) != nil {
		return value, errors.New("outbox_heartbeat_unavailable")
	}
	if value.State != "running" || value.ErrorCode != "" || value.CheckedAt.IsZero() || value.CheckedAt.After(now.Add(time.Second)) || now.Sub(value.CheckedAt) > 45*time.Second {
		return value, errors.New("outbox_not_healthy")
	}
	return value, nil
}
