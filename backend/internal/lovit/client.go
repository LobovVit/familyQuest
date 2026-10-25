// Package lovit reads product-scoped rights; it never confirms or copies payments.
package lovit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/lobov/familyquest/backend/internal/domain"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrUnavailable = errors.New("LOVIT entitlement unavailable")

type Entitlement = domain.LOVITEntitlement

type Client struct {
	endpoint, token string
	http            *http.Client
}

func New(origin, token string) (*Client, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))) || len(token) < 32 || len(token) > 512 || strings.ContainsAny(token, "\r\n") {
		return nil, ErrUnavailable
	}
	u.Path = "/v1/entitlements/check"
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &Client{u.String(), token, &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) Check(ctx context.Context, subject, subscription string) (Entitlement, error) {
	var e Entitlement
	b, _ := json.Marshal(map[string]string{"subjectRef": subject})
	r, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(b))
	if err != nil {
		return e, ErrUnavailable
	}
	r.Header.Set("Authorization", "Bearer "+c.token)
	r.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(r)
	if err != nil {
		return e, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return e, ErrUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 16385))
	if err != nil || len(data) > 16384 {
		return Entitlement{}, ErrUnavailable
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(&e) != nil || d.Decode(new(any)) != io.EOF {
		return Entitlement{}, ErrUnavailable
	}
	if err = e.Validate(subject, subscription, time.Now()); err != nil {
		return Entitlement{}, err
	}
	return e, nil
}
