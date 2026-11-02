package access

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TelegramPortal is the Portal v1 boundary. No bot credentials or identity matching live here.
type TelegramPortal interface {
	Call(context.Context, string, any, any) error
}

type TelegramPrincipal struct {
	ProductID     string `json:"productId"`
	BotID         string `json:"botId"`
	IdentityState string `json:"identityState"`
	Issuer        string `json:"issuer"`
	Subject       string `json:"subject"`
	RecipientRef  string `json:"recipientRef"`
}
type telegramExchange struct {
	SessionToken string            `json:"sessionToken"`
	ExpiresAt    time.Time         `json:"expiresAt"`
	Principal    TelegramPrincipal `json:"principal"`
}
type telegramPortalError struct {
	status int
	code   string
}

func (e telegramPortalError) Error() string { return "Telegram Portal request failed" }

type telegramHTTP struct {
	origin, key string
	client      *http.Client
}

func NewTelegramPortal(origin, key string) (TelegramPortal, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))) || len(key) < 32 {
		return nil, errors.New("Telegram requires a Portal HTTPS origin and dedicated product service key")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &telegramHTTP{strings.TrimRight(origin, "/"), key, &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (p *telegramHTTP) Call(ctx context.Context, route string, input, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.origin+"/v1/telegram/"+route, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.key)
	req.Header.Set("Content-Type", "application/json")
	res, err := p.client.Do(req)
	if err != nil {
		return errors.New("Telegram Portal unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 202 {
		var failure struct {
			Code string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&failure)
		return telegramPortalError{status: res.StatusCode, code: failure.Code}
	}
	if output == nil {
		_, err = io.Copy(io.Discard, io.LimitReader(res.Body, 65536))
		return err
	}
	d := json.NewDecoder(io.LimitReader(res.Body, 65536))
	if err = d.Decode(output); err != nil {
		return errors.New("invalid Telegram Portal response")
	}
	return nil
}
