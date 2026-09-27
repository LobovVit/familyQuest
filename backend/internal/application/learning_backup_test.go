package application

import (
	"fmt"
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

func TestLearningBackupRewardPolicyCompatibility(t *testing.T) {
	settings := domain.MathSettings{Operation: "+", Level: "hard", AnswerMode: "input", DivisionMode: "result"}
	session := domain.NewMathSession("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1, settings, time.Now())
	session.RewardVersion = 0
	for i, q := range session.Questions {
		_, answer := q.Work(settings)
		if err := session.Submit(i, answer, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	b := BackupData{Version: 3, Participants: []BackupParticipant{{ID: 1, Role: domain.RoleChild}}, MathSessions: []domain.MathSession{session}}
	for _, a := range session.Answers {
		b.ActivityRewards = append(b.ActivityRewards, domain.ActivityReward{Source: "math", SourceKey: fmt.Sprintf("%s:%d", session.ID, a.Index), ParticipantID: 1, Date: a.Date, Stars: a.Stars, Title: "Math"})
	}
	if err := b.validateLearning(); err != nil {
		t.Fatal("old uncapped backup", err)
	}
	b.Version = BackupVersion
	if err := b.validateLearning(); err != nil {
		t.Fatal("old rewards re-export", err)
	}
	b.MathSessions[0].RewardVersion = 2
	if err := b.validateLearning(); err == nil {
		t.Fatal("accepted new rewards above daily budget")
	}
	b.MathSessions[0].RewardVersion = 99
	if err := b.validateLearning(); err == nil {
		t.Fatal("accepted unknown policy")
	}
}

func TestReadingBackupValidation(t *testing.T) {
	c := domain.ReadingCompletion{ID: "0123456789abcdef0123456789abcdef", Level: "advanced"}
	r := c.Reward(1, 0, time.Now())
	valid := func() BackupData {
		return BackupData{Version: BackupVersion, Participants: []BackupParticipant{{ID: 1, Role: domain.RoleChild}}, ActivityRewards: []domain.ActivityReward{r}}
	}
	b := valid()
	if b.validateLearning() != nil {
		t.Fatal("valid")
	}
	for _, mutate := range []func(*BackupData){
		func(b *BackupData) { b.Version = 4 },
		func(b *BackupData) { b.ActivityRewards[0].Stars = 13 },
		func(b *BackupData) { b.ActivityRewards[0].SourceKey = "bad" },
		func(b *BackupData) { b.ActivityRewards[0].Smiles = 1 },
		func(b *BackupData) { b.Participants[0].Role = domain.RoleParent },
		func(b *BackupData) { b.ActivityRewards = append(b.ActivityRewards, r) },
		func(b *BackupData) {
			for i := 0; i < 3; i++ {
				copy := r
				copy.SourceKey = fmt.Sprintf("%032x", i)
				b.ActivityRewards = append(b.ActivityRewards, copy)
			}
		},
	} {
		b = valid()
		mutate(&b)
		if b.validateLearning() == nil {
			t.Fatal("invalid reading backup", b)
		}
	}
	b = valid()
	b.ActivityRewards[0].Stars = 0
	if b.validateLearning() != nil {
		t.Fatal("zero completion")
	}
}
