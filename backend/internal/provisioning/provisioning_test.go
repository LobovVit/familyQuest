package provisioning

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lobov/familyquest/backend/internal/domain"
)

type registrarFunc func(context.Context, string, string, domain.FamilyRegistration) (int64, error)

func (f registrarFunc) RegisterFamily(c context.Context, i, s string, v domain.FamilyRegistration) (int64, error) {
	return f(c, i, s, v)
}
func TestPrivateRegistrationBoundary(t *testing.T) {
	calls := 0
	secret := strings.Repeat("p", 40)
	handler, err := Handler(registrarFunc(func(_ context.Context, issuer, subject string, in domain.FamilyRegistration) (int64, error) {
		calls++
		if issuer != "https://id.example" || subject != "person" || in.Name != "Семья" {
			t.Error("incorrect identity/input")
		}
		return 19, nil
	}), "https://id.example", secret)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client, err := NewClient(server.URL, secret)
	if err != nil {
		t.Fatal(err)
	}
	if id, err := client.RegisterFamily(context.Background(), "https://id.example", "person", domain.FamilyRegistration{Name: " Семья ", ParentName: "Анна", PIN: "739281"}); err != nil || id != 19 {
		t.Fatal(id, err)
	}
	for _, test := range []struct {
		key, body string
		status    int
	}{
		{"", `{}`, 401}, {strings.Repeat("c", 40), `{}`, 401},
		{secret, `{"issuer":"https://other.example","subject":"person","family":{"name":"Семья","parentName":"Анна","pin":"739281"}}`, 400},
		{secret, `{"issuer":"https://id.example","subject":"person","family":{"name":"Семья","parentName":"Анна","pin":"739281","familyId":1}}`, 400},
		{secret, `{"issuer":"https://id.example","subject":"person","family":{"name":"Семья","parentName":"Анна","pin":"739281"}} {}`, 400},
	} {
		r := httptest.NewRequest("POST", Path, strings.NewReader(test.body))
		r.Header.Set("Authorization", "Bearer "+test.key)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != test.status {
			t.Fatal(w.Code, test.status)
		}
	}
	if calls != 1 {
		t.Fatal("unauthorized provisioner call", calls)
	}
}
