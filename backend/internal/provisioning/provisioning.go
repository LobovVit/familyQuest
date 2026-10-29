// Package provisioning is the private, separately credentialed family creation boundary.
package provisioning

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lobov/familyquest/backend/internal/domain"
)

const Path = "/internal/families"

type Registrar interface {
	RegisterFamily(context.Context, string, string, domain.FamilyRegistration) (int64, error)
}
type request struct {
	Issuer  string                    `json:"issuer"`
	Subject string                    `json:"subject"`
	Family  domain.FamilyRegistration `json:"family"`
}
type result struct {
	FamilyID int64 `json:"familyId"`
}

func Handler(registrar Registrar, issuer, secret string) (http.Handler, error) {
	if registrar == nil || issuer == "" || len(secret) < 32 {
		return nil, errors.New("private registration configuration required")
	}
	expected := sha256.Sum256([]byte("Bearer " + secret))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path != Path || r.URL.RawPath != "" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		actual := sha256.Sum256([]byte(r.Header.Get("Authorization")))
		if len(r.Header.Values("Authorization")) != 1 || subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 {
			w.WriteHeader(401)
			return
		}
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			w.WriteHeader(415)
			return
		}
		var in request
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&in) != nil || decoder.Decode(new(any)) != io.EOF || in.Issuer != issuer || strings.TrimSpace(in.Subject) == "" || len(in.Subject) > 512 {
			w.WriteHeader(400)
			return
		}
		in.Family, err = in.Family.Normalize()
		if err != nil {
			w.WriteHeader(400)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		defer cancel()
		family, err := registrar.RegisterFamily(ctx, in.Issuer, in.Subject, in.Family)
		switch {
		case errors.Is(err, domain.ErrInvalidInput):
			w.WriteHeader(400)
		case errors.Is(err, domain.ErrForbidden), errors.Is(err, domain.ErrConflict):
			w.WriteHeader(403)
		case err != nil:
			w.WriteHeader(503)
		default:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(result{family})
		}
	}), nil
}

type Client struct {
	origin, secret string
	http           *http.Client
}

func NewClient(origin, secret string) (*Client, error) {
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || len(secret) < 32 {
		return nil, errors.New("private provisioning origin and dedicated secret required")
	}
	u.Path = Path
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &Client{u.String(), secret, &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) RegisterFamily(ctx context.Context, issuer, subject string, family domain.FamilyRegistration) (int64, error) {
	body, err := json.Marshal(request{issuer, subject, family})
	if err != nil {
		return 0, err
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, c.origin, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	r.Header.Set("Authorization", "Bearer "+c.secret)
	r.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(r)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case 400:
		return 0, domain.ErrInvalidInput
	case 403:
		return 0, domain.ErrForbidden
	case 200:
		var out result
		if json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&out) == nil && out.FamilyID > 0 {
			return out.FamilyID, nil
		}
	}
	return 0, errors.New("family registration unavailable")
}
