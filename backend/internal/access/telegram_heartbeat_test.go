package access

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOutboxHeartbeatRejectsStaleStoppedAndFailedWorkers(t *testing.T) {
	now := time.Now().UTC()
	path := filepath.Join(t.TempDir(), "status.json")
	if _, err := ReadTelegramOutboxHeartbeat(path, now); err == nil {
		t.Fatal("absent heartbeat accepted")
	}
	for _, tc := range []struct {
		state, code string
		age         time.Duration
		ok          bool
	}{
		{"running", "", 10 * time.Second, true},
		{"running", "", 46 * time.Second, false},
		{"running", "", -5 * time.Second, false},
		{"disabled", "", 0, false},
		{"starting", "", 0, false},
		{"stopped", "", 0, false},
		{"running", "catalog_unavailable", 0, false},
	} {
		if err := WriteTelegramOutboxHeartbeat(path, TelegramOutboxHeartbeat{now.Add(-tc.age), tc.state, tc.code}); err != nil {
			t.Fatal(err)
		}
		_, err := ReadTelegramOutboxHeartbeat(path, now)
		if (err == nil) != tc.ok {
			t.Fatal(tc, err)
		}
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("heartbeat permissions", info.Mode())
	}
}

type heartbeatCatalog struct{ err error }

func (c heartbeatCatalog) NotificationFamilies(context.Context, int64) ([]int64, error) {
	return nil, c.err
}

func TestOutboxLoopReportsFailureAndGracefulStop(t *testing.T) {
	for _, broken := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		var states []TelegramOutboxHeartbeat
		catalog := heartbeatCatalog{}
		if broken {
			catalog.err = errors.New("database unavailable")
		}
		g := &Gateway{}
		g.RunTelegramOutbox(ctx, catalog, func(value TelegramOutboxHeartbeat) {
			states = append(states, value)
			if value.State == "running" {
				cancel()
			}
		})
		cancel()
		if len(states) != 3 || states[0].State != "starting" || states[1].State != "running" || states[2].State != "stopped" || (states[1].ErrorCode != "") != broken {
			t.Fatal(states)
		}
	}
}
