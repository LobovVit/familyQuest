package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lobov/familyquest/backend/internal/domain"
)

type Tokens struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}
type claims struct {
	Subject string `json:"sub"`
	Role    string `json:"role"`
	Expires int64  `json:"exp"`
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
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	b, err := json.Marshal(claims{Subject: strconv.FormatInt(p.ID, 10), Role: p.Role, Expires: t.now().Add(t.ttl).Unix()})
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
	if json.Unmarshal(b, &c) != nil || c.Expires <= t.now().Unix() {
		return domain.Principal{}, domain.ErrUnauthorized
	}
	id, err := strconv.ParseInt(c.Subject, 10, 64)
	if err != nil || domain.ValidateRole(c.Role) != nil {
		return domain.Principal{}, domain.ErrUnauthorized
	}
	return domain.Principal{ParticipantID: id, Role: c.Role}, nil
}
func Bearer(header string) (string, error) {
	p := strings.Fields(header)
	if len(p) != 2 || !strings.EqualFold(p[0], "Bearer") || p[1] == "" {
		return "", fmt.Errorf("%w", domain.ErrUnauthorized)
	}
	return p[1], nil
}
