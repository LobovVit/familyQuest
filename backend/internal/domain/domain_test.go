package domain

import "testing"

func TestPINAndCompletionPolicy(t *testing.T) {
	if ValidatePIN("123456") != nil {
		t.Fatal("valid pin rejected")
	}
	for _, v := range []string{"12345", "abcdef", "１２３４５６"} {
		if ValidatePIN(v) == nil {
			t.Fatalf("invalid pin %q accepted", v)
		}
	}
	p := Principal{ParticipantID: 2, Role: RoleChild}
	if !CanComplete(p, 2) || CanComplete(p, 3) {
		t.Fatal("completion ownership policy failed")
	}
}
