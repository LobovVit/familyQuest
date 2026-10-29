package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// FamilyRegistration contains no billing, tenant identifiers or account credentials.
type FamilyRegistration struct {
	Name       string `json:"name"`
	ParentName string `json:"parentName"`
	PIN        string `json:"pin"`
}

func (v FamilyRegistration) Normalize() (FamilyRegistration, error) {
	v.Name = strings.TrimSpace(v.Name)
	v.ParentName = strings.TrimSpace(v.ParentName)
	if err := ValidateFamilyName(v.Name); err != nil {
		return v, err
	}
	if err := ValidateFamilyName(v.ParentName); err != nil {
		return v, err
	}
	if ValidatePIN(v.PIN) != nil {
		return v, ErrInvalidInput
	}
	return v, nil
}

func ValidateFamilyName(name string) error {
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 80 {
		return ErrInvalidInput
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return ErrInvalidInput
		}
	}
	return nil
}
