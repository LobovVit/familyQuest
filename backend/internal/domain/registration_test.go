package domain

import (
	"strings"
	"testing"
)

func TestFamilyRegistrationValidation(t *testing.T) {
	valid := FamilyRegistration{Name: "  Соколовы  ", ParentName: "  Анна  ", PIN: "739281"}
	got, err := valid.Normalize()
	if err != nil || got.Name != "Соколовы" || got.ParentName != "Анна" {
		t.Fatal(got, err)
	}
	for _, input := range []FamilyRegistration{
		{Name: "   ", ParentName: "Анна", PIN: valid.PIN},
		{Name: strings.Repeat("я", 81), ParentName: "Анна", PIN: valid.PIN},
		{Name: "Семья", ParentName: "   ", PIN: valid.PIN},
		{Name: "Семья", ParentName: "Ан\nна", PIN: valid.PIN},
		{Name: "Семья", ParentName: "Анна", PIN: "12345"},
		{Name: "Семья", ParentName: "Анна", PIN: "１２３４５６"},
	} {
		if _, err := input.Normalize(); err == nil {
			t.Errorf("accepted invalid registration: %#v", input)
		}
	}
	valid.Name = strings.Repeat("я", 80)
	if _, err := valid.Normalize(); err != nil {
		t.Fatal("unicode name limit", err)
	}
}
