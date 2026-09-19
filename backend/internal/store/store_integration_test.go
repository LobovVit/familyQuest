package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lobov/familyquest/backend/internal/application"
	"github.com/lobov/familyquest/backend/internal/auth"
	"github.com/lobov/familyquest/backend/internal/domain"
)

// FAMILYQUEST_TEST_DATABASE_URL must point to a disposable database.
func TestPostgresWorkflows(t *testing.T) {
	url := os.Getenv("FAMILYQUEST_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set FAMILYQUEST_TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	ctx := context.Background()
	s, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// Migrate resolves paths relative to the repository root or backend directory.
	t.Chdir("../..")
	migrationResults := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { migrationResults <- s.Migrate(ctx) }()
	}
	for i := 0; i < 2; i++ {
		if err := <-migrationResults; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.pool.Exec(ctx, `truncate participants,chores,rewards cascade`); err != nil {
		t.Fatal(err)
	}
	for name, list := range map[string]func() (any, error){
		"participants": func() (any, error) { return s.ListParticipants(ctx) },
		"chores":       func() (any, error) { return s.ListChores(ctx) },
		"rewards":      func() (any, error) { return s.ListRewards(ctx) },
		"assignments":  func() (any, error) { return s.ListAssignments(ctx) },
		"tasks":        func() (any, error) { return s.ListTasks(ctx, time.Now()) },
		"leaderboard":  func() (any, error) { return s.Leaderboard(ctx, "day", time.Now()) },
	} {
		v, e := list()
		if e != nil {
			t.Fatal(e)
		}
		b, _ := json.Marshal(v)
		if string(b) != "[]" {
			t.Fatalf("%s: empty list is %s", name, b)
		}
	}
	parent, err := s.CreateParticipant(ctx, domain.Participant{Name: "Review parent", Role: domain.RoleParent}, "739281")
	if err != nil {
		t.Fatal(err)
	}
	child, err := s.CreateParticipant(ctx, domain.Participant{Name: "Review child", Role: domain.RoleChild}, "391827")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.VerifyParticipantPIN(ctx, child.ID, "391827"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.VerifyParticipantPIN(ctx, child.ID, "000000"); !errors.Is(err, domain.ErrInvalidPIN) {
		t.Fatalf("wrong PIN: %v", err)
	}

	if err := s.DeleteParticipant(ctx, parent.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("last parent deletion: %v", err)
	}
	if _, err := s.CreateParticipant(ctx, domain.Participant{Name: parent.Name, Role: domain.RoleChild}, "111111"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate name overwrote participant: %v", err)
	}
	tokens, err := auth.New("review-only-token-secret-at-least-32-characters", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	app := application.New(s, tokens)
	_, oldToken, err := app.Authenticate(ctx, child.ID, "391827")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateParticipantPIN(ctx, child.ID, "391827"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ParseToken(ctx, oldToken); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("PIN change retained session: %v", err)
	}
	_, beforeRestore, err := app.Authenticate(ctx, child.ID, "391827")
	if err != nil {
		t.Fatal(err)
	}
	for _, schedule := range []string{"daily", "weekly", "monthly", "once"} {
		t.Run(schedule, func(t *testing.T) {
			chore, err := s.CreateChore(ctx, domain.Chore{Title: schedule, Schedule: schedule, BaseValue: 50, ParticipantIDs: []int64{child.ID}})
			if err != nil {
				t.Fatal(err)
			}
			tasks, err := s.ListTasks(ctx, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			var task domain.Task
			for _, v := range tasks {
				if v.ChoreID == chore.ID {
					task = v
				}
			}
			if task.ID == 0 {
				t.Fatal("task was not generated")
			}
			if _, err := s.ConfirmTask(ctx, task.ID, parent.ID, 5, ""); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("pending confirmation: %v", err)
			}
			var count int
			if err := s.pool.QueryRow(ctx, `select count(*) from confirmations where task_id=$1`, task.ID).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatal("pending task gained a confirmation")
			}
			if _, err := s.CompleteTask(ctx, task.ID, parent.ID); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("wrong owner completion: %v", err)
			}
			if _, err := s.CompleteTask(ctx, task.ID, child.ID); err != nil {
				t.Fatal(err)
			}
			confirmed, err := s.ConfirmTask(ctx, task.ID, parent.ID, 5, "")
			if err != nil {
				t.Fatal(err)
			}
			if confirmed.Status != "confirmed" || confirmed.Reward != 50 {
				t.Fatalf("unexpected confirmed task: %+v", confirmed)
			}
		})
	}
	backup, err := s.ExportBackup(ctx)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(backup)
	if strings.Contains(string(payload), "pinCode") || strings.Contains(string(payload), "391827") {
		t.Fatal("backup leaked credentials")
	}
	if err := s.ImportBackup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	if _, err := s.VerifyParticipantPIN(ctx, child.ID, "391827"); err != nil {
		t.Fatalf("restore lost PIN: %v", err)
	}
	if _, err := app.ParseToken(ctx, beforeRestore); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("restore retained session: %v", err)
	}
	broken := backup
	broken.Tasks = append([]BackupTask(nil), backup.Tasks...)
	broken.Tasks[0].AssignmentID = -1
	if err := s.ImportBackup(ctx, broken); err == nil {
		t.Fatal("invalid foreign key accepted")
	}
	if _, err := s.VerifyParticipantPIN(ctx, parent.ID, "739281"); err != nil {
		t.Fatalf("rollback lost parent: %v", err)
	}
	secondParent, err := s.CreateParticipant(ctx, domain.Participant{Name: "Second parent", Role: domain.RoleParent}, "619283")
	if err != nil {
		t.Fatal(err)
	}
	deletions := make(chan error, 2)
	for _, id := range []int64{parent.ID, secondParent.ID} {
		go func(id int64) { deletions <- s.DeleteParticipant(ctx, id) }(id)
	}
	conflicts := 0
	for i := 0; i < 2; i++ {
		err := <-deletions
		if errors.Is(err, domain.ErrConflict) {
			conflicts++
		} else if err != nil {
			t.Fatal(err)
		}
	}
	if conflicts != 1 {
		t.Fatalf("concurrent deletions should preserve one parent; conflicts=%d", conflicts)
	}
	var parents int
	if err := s.pool.QueryRow(ctx, `select count(*) from participants where active and role='parent'`).Scan(&parents); err != nil {
		t.Fatal(err)
	}
	if parents != 1 {
		t.Fatalf("active parents after concurrent deletions: %d", parents)
	}

	if ResolveSeedPath("") != "" {
		t.Fatal("implicit seed enabled")
	}
}
