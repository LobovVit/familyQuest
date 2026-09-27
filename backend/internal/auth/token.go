package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/lobov/familyquest/backend/internal/domain"
)

type Tokens struct {
	familyID int64
	secret   []byte
	ttl      time.Duration
	now      func() time.Time
}
type claims struct {
	FamilyID       int64  `json:"familyId,omitempty"`
	DeviceID       string `json:"device,omitempty"`
	ConfirmedUntil int64  `json:"confirmedUntil,omitempty"`
	Version        int64  `json:"ver"`
	Subject        string `json:"sub"`
	Role           string `json:"role"`
	Expires        int64  `json:"exp"`
}

func New(secret string, ttl time.Duration) (*Tokens, error) {
	if len(secret) < 32 {
		return nil, errors.New("SESSION_SECRET must be at least 32 characters")
	}
	if ttl <= 0 {
		return nil, errors.New("session ttl must be positive")
	}
	return &Tokens{secret: []byte(secret), ttl: ttl, now: time.Now}, nil
}
func (t *Tokens) Issue(p domain.Participant) (string, error) {
	return t.issue(claims{FamilyID: t.familyID, Version: p.SessionVersion, Subject: strconv.FormatInt(p.ID, 10), Role: p.Role, Expires: t.now().Add(t.ttl).Unix()})
}
func (t *Tokens) IssueConfirmation(p domain.Principal) (string, error) {
	expires := t.now().Add(5 * time.Minute).Unix()
	return t.issue(claims{FamilyID: t.familyID, Version: p.SessionVersion, Subject: strconv.FormatInt(p.ParticipantID, 10), Role: p.Role, Expires: expires, ConfirmedUntil: expires, DeviceID: p.DeviceID})
}
func (t *Tokens) issue(c claims) (string, error) {
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(b)
	unsigned := h + "." + payload
	mac := hmac.New(sha256.New, t.secret)
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func (t *Tokens) Parse(token string) (domain.Principal, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return domain.Principal{}, domain.ErrUnauthorized
	}
	mac := hmac.New(sha256.New, t.secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
		return domain.Principal{}, domain.ErrUnauthorized
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return domain.Principal{}, domain.ErrUnauthorized
	}
	var c claims
	if json.Unmarshal(b, &c) != nil || c.Expires <= t.now().Unix() || (t.familyID > 0 && c.FamilyID != t.familyID) {
		return domain.Principal{}, domain.ErrUnauthorized
	}
	id, err := strconv.ParseInt(c.Subject, 10, 64)
	if err != nil || id <= 0 || domain.ValidateRole(c.Role) != nil {
		return domain.Principal{}, domain.ErrUnauthorized
	}
	return domain.Principal{FamilyID: c.FamilyID, ParticipantID: id, Role: c.Role, SessionVersion: c.Version, DeviceID: c.DeviceID, ConfirmedUntil: c.ConfirmedUntil}, nil
}

// ForFamily binds both issuance and parsing to one family.
// ForFamily связывает выпуск и проверку токенов с одной семьёй.
func (t *Tokens) ForFamily(id int64) *Tokens { v := *t; v.familyID = id; return &v }
