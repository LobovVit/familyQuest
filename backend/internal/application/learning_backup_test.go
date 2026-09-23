package application

import (
	"github.com/lobov/familyquest/backend/internal/domain"
	"testing"
	"time"
)

func TestLearningBackupSingleActiveSession(t *testing.T) {
	settings := domain.MathSettings{Operation: "+", Level: "easy", AnswerMode: "input", DivisionMode: "result"}
	first := domain.NewMathSession("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1, settings, time.Now())
	second := domain.NewMathSession("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 1, settings, time.Now())
	backup := BackupData{Version: BackupVersion, Participants: []BackupParticipant{{ID: 1, Role: domain.RoleChild}}, MathSessions: []domain.MathSession{first, second}}
	if err := backup.validateLearning(); err == nil {
		t.Fatal("accepted two active sessions for one child")
	}
	backup.MathSessions[0].Closed = true
	if err := backup.validateLearning(); err != nil {
		t.Fatalf("closed history plus active session: %v", err)
	}
}
