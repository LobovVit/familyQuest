package auth

import (
	"testing"
	"time"

	"github.com/lobov/familyquest/backend/internal/domain"
)

func TestTokenRoundTripAndTampering(t *testing.T) {
	tokens, err := New("01234567890123456789012345678901", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, err := tokens.Issue(domain.Participant{ID: 7, Role: domain.RoleChild})
	if err != nil {
		t.Fatal(err)
	}
	p, err := tokens.Parse(token)
	if err != nil || p.ParticipantID != 7 || p.Role != domain.RoleChild {
		t.Fatalf("unexpected principal %#v, %v", p, err)
	}
	if _, err = tokens.Parse(token + "x"); err == nil {
		t.Fatal("tampered token accepted")
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	tokens, _ := New("01234567890123456789012345678901", time.Second)
	now := time.Unix(100, 0)
	tokens.now = func() time.Time { return now }
	token, _ := tokens.Issue(domain.Participant{ID: 1, Role: domain.RoleParent})
	tokens.now = func() time.Time { return now.Add(2 * time.Second) }
	if _, err := tokens.Parse(token); err == nil {
		t.Fatal("expired token accepted")
	}
}

func TestConfirmationBoundToDeviceAndExpires(t *testing.T) {
	tokens, _ := New("01234567890123456789012345678901", 12*time.Hour)
	now := time.Now()
	tokens.now = func() time.Time { return now }
	proof, err := tokens.IssueConfirmation(domain.Principal{ParticipantID: 1, Role: domain.RoleParent, DeviceID: "ipad", SessionVersion: 7})
	if err != nil {
		t.Fatal(err)
	}
	p, err := tokens.Parse(proof)
	if err != nil || p.DeviceID != "ipad" || p.SessionVersion != 7 || p.ConfirmedUntil != now.Add(5*time.Minute).Unix() {
		t.Fatal("proof claims", p, err)
	}
	tokens.now = func() time.Time { return now.Add(5 * time.Minute) }
	if _, err = tokens.Parse(proof); err == nil {
		t.Fatal("expired confirmation accepted")
	}
}
