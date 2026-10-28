package access

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type summaryCatalog struct{ lookupErr, paidErr error }

func (c summaryCatalog) PortalFamily(context.Context, string, string) (int64, error) {
	return 7, c.lookupErr
}
func (c summaryCatalog) Paid(context.Context, int64) (bool, error) { return false, c.paidErr }

func TestPortalSummaryFailsClosedAndCannotForwardCredentials(t *testing.T) {
	for _, tc := range []struct {
		name, body         string
		status             int
		lookupErr, paidErr error
	}{
		{name: "core unavailable", status: 503, body: `{"internal":"database details"}`},
		{name: "wrong family", status: 200, body: `{"familyId":8}`},
		{name: "malformed", status: 200, body: `invalid`},
		{name: "catalog unavailable", lookupErr: errors.New("private database failure")},
		{name: "payment lookup unavailable", status: 200, body: `{"familyId":7}`, paidErr: errors.New("private payment failure")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Error("reader forwarded user/service credentials")
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer core.Close()
			g, _ := New(fakeCatalog{}, core.URL, strings.Repeat("c", 40), "")
			g.Portal, _ = NewPortalReader(summaryCatalog{tc.lookupErr, tc.paidErr}, "https://issuer.example", strings.Repeat("p", 40))
			r := httptest.NewRequest("POST", "https://family.example"+PortalAccountPath, strings.NewReader(`{"issuer":"https://issuer.example","subject":"person"}`))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Authorization", "Bearer "+strings.Repeat("p", 40))
			r.Header.Set("Cookie", "private=old-session")
			w := httptest.NewRecorder()
			g.ServeHTTP(w, r)
			if w.Code != 503 || strings.Contains(w.Body.String(), "private") || strings.Contains(w.Body.String(), "connected") {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
}
