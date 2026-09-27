package identity

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

// LogoutURL uses only discovery metadata and the configured application origin.
// LogoutURL использует только discovery и настроенный адрес приложения.
func (l *Login) LogoutURL() (string, error) {
	if l.endSession == "" {
		return "", errors.New("provider logout unavailable")
	}
	u, err := url.Parse(l.endSession)
	if err != nil {
		return "", err
	}
	app, _ := url.Parse(l.config.RedirectURL)
	q := u.Query()
	q.Set("client_id", l.config.ClientID)
	q.Set("post_logout_redirect_uri", app.Scheme+"://"+app.Host+"/")
	u.RawQuery = q.Encode()
	return u.String(), nil
}
func (l *Login) VerifyLogout(r *http.Request) (Subject, error) {
	fail := errors.New("invalid logout token")
	if r.Method != "POST" {
		return Subject{}, fail
	}
	r.Body = http.MaxBytesReader(nil, r.Body, 64<<10)
	if r.ParseForm() != nil || len(r.PostForm["logout_token"]) != 1 {
		return Subject{}, fail
	}
	id, err := l.verifier.Verify(oidc.ClientContext(r.Context(), l.client), r.PostForm.Get("logout_token"))
	if err != nil {
		return Subject{}, fail
	}
	var c struct {
		SessionID string                     `json:"sid"`
		TokenID   string                     `json:"jti"`
		Issued    int64                      `json:"iat"`
		Nonce     json.RawMessage            `json:"nonce"`
		Events    map[string]json.RawMessage `json:"events"`
	}
	if id.Claims(&c) != nil || c.TokenID == "" || len(c.Nonce) > 0 || (id.Subject == "" && c.SessionID == "") || len(c.SessionID) > 512 || len(id.Subject) > 512 {
		return Subject{}, fail
	}
	event, ok := c.Events["http://schemas.openid.net/event/backchannel-logout"]
	var object map[string]json.RawMessage
	if !ok || json.Unmarshal(event, &object) != nil || object == nil || len(object) != 0 {
		return Subject{}, fail
	}
	now := time.Now()
	issued := time.Unix(c.Issued, 0)
	if issued.After(now.Add(time.Minute)) || issued.Before(now.Add(-16*time.Minute)) {
		return Subject{}, fail
	}
	return Subject{Issuer: l.config.Issuer, ID: id.Subject, SessionID: c.SessionID}, nil
}
