package access

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const readyTelegram = `{"scope":"portal_telegram","productId":"familyquest","configured":true,"state":"ready","ready":true,"reasons":[],"incoming":{"mode":"polling","ready":true},"delivery":{"mode":"linked","ready":true},"queue":{"pending":0,"sending":0,"uncertain":0,"failed":0,"oldestDueSeconds":0,"oldestSendingSeconds":0}}`

func TestTelegramReadinessFailsClosed(t *testing.T) {
	for _, tc := range []struct{ name, from, to string }{
		{"foreign scope", "portal_telegram", "web_health"},
		{"foreign product", "familyquest", "lovit-dom"},
		{"unknown state", `"state":"ready"`, `"state":"new"`},
		{"false positive", `"configured":true`, `"configured":false`},
		{"missing flag", `"configured":true,`, ""},
		{"null flag", `"configured":true`, `"configured":null`},
		{"wrong type", `"configured":true`, `"configured":"true"`},
		{"transport missing", `"mode":"polling","ready":true`, `"mode":"polling"`},
		{"manual transport", `"polling"`, `"manual"`},
		{"delivery disabled", `"linked"`, `"disabled"`},
		{"negative queue", `"pending":0`, `"pending":-1`},
		{"missing queue field", `"pending":0,`, ""},
		{"unknown delivery", `"linked"`, `"newmode"`},
		{"uncertain queue", `"uncertain":0`, `"uncertain":1`},
		{"failed queue", `"failed":0`, `"failed":1`},
		{"recent failure", `"failed":0`, `"failed":1,"recentFailed":1`},
		{"negative recent failure", `"failed":0`, `"failed":1,"recentFailed":-1`},
		{"overdue pending", `"oldestDueSeconds":0`, `"oldestDueSeconds":301`},
		{"stuck sender", `"oldestSendingSeconds":0`, `"oldestSendingSeconds":31`},
		{"unexpected reason", `"reasons":[]`, `"reasons":["private provider error"]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := decodeTelegramReadiness([]byte(strings.Replace(readyTelegram, tc.from, tc.to, 1))); err == nil {
				t.Fatal("unsafe readiness accepted")
			}
		})
	}
	for _, body := range []string{readyTelegram, strings.Replace(readyTelegram, `"configured":true`, `"configured":true,"futureField":42`, 1), strings.Replace(readyTelegram, `"failed":0`, `"failed":1,"recentFailed":0`, 1)} {
		if value, err := decodeTelegramReadiness([]byte(body)); err != nil || !value.Ready {
			t.Fatal(value, err)
		}
	}
	var disabled map[string]any
	_ = json.Unmarshal([]byte(readyTelegram), &disabled)
	disabled["state"], disabled["ready"], disabled["configured"], disabled["reasons"] = "unconfigured", false, false, []string{"bot_unconfigured"}
	body, _ := json.Marshal(disabled)
	if value, err := decodeTelegramReadiness(body); err != nil || value.Ready {
		t.Fatal("disabled dependency is a valid non-ready response", value, err)
	}
}

func TestTelegramReadinessOnlyReadsItsProductAndNeverFollowsRedirects(t *testing.T) {
	key := strings.Repeat("k", 40)
	redirected := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected = true }))
	defer destination.Close()
	for _, code := range []int{200, 401, 503, 302} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" || r.URL.Path != "/v1/telegram/readiness" || r.URL.RawQuery != "" || r.Header.Get("Authorization") != "Bearer "+key || r.Header.Get("Cookie") != "" {
				t.Error("invalid scoped readiness request")
			}
			w.Header().Set("Location", destination.URL)
			w.WriteHeader(code)
			if code == 200 {
				_, _ = w.Write([]byte(readyTelegram))
			} else {
				_, _ = w.Write([]byte("provider secret must never escape"))
			}
		}))
		_, err := ReadTelegramReadiness(context.Background(), server.URL, key)
		server.Close()
		if (err == nil) != (code == 200) || err != nil && strings.Contains(err.Error(), "secret") {
			t.Fatal(code, err)
		}
	}
	if redirected {
		t.Fatal("credential-bearing readiness followed redirect")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := ReadTelegramReadiness(ctx, server.URL, key); err == nil {
		t.Fatal("timeout claimed readiness")
	}
}
