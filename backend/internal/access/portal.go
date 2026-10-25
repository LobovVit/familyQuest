package access

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lobov/familyquest/backend/internal/domain"
	"github.com/lobov/familyquest/backend/internal/httpapi"
)

const PortalAccountPath = "/api/integrations/lovit/account"
const PortalBillingPath = "/api/integrations/lovit/billing-context"

type PortalCatalog interface {
	PortalFamily(context.Context, string, string) (int64, error)
	Paid(context.Context, int64) (bool, error)
}
type PortalReader struct {
	catalog PortalCatalog
	issuer  string
	keyHash [32]byte
}

// PortalSubscription excludes email, profiles, tokens and individual payments.
type PortalSubscription struct {
	ChildrenUnlimited bool       `json:"childrenUnlimited,omitempty"`
	BillingManaged    bool       `json:"billingManaged,omitempty"`
	Name              string     `json:"name"`
	Status            string     `json:"status"`
	Plan              string     `json:"plan"`
	ChildLimit        int        `json:"childLimit"`
	ActiveChildren    int        `json:"activeChildren"`
	AccessUntil       *time.Time `json:"accessUntil"`
	Paid              bool       `json:"paid"`
	Access            bool       `json:"access"`
}

func NewPortalReader(catalog PortalCatalog, issuer, key string) (*PortalReader, error) {
	u, err := url.Parse(issuer)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))) || len(key) < 32 || len(key) > 512 || catalog == nil {
		return nil, errors.New("portal reader requires an OIDC issuer and a dedicated service key")
	}
	return &PortalReader{catalog: catalog, issuer: issuer, keyHash: sha256.Sum256([]byte(key))}, nil
}

func (g *Gateway) portalAccount(w http.ResponseWriter, r *http.Request) {
	p := g.Portal
	if p == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	auth := r.Header.Get("Authorization")
	hash := sha256.Sum256([]byte(strings.TrimPrefix(auth, "Bearer ")))
	if len(r.Header.Values("Authorization")) != 1 || !strings.HasPrefix(auth, "Bearer ") || subtle.ConstantTimeCompare(hash[:], p.keyHash[:]) != 1 {
		fail(w, 401, "Service authentication required")
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		fail(w, 415, "JSON required")
		return
	}
	var in struct {
		Issuer  string `json:"issuer"`
		Subject string `json:"subject"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil || d.Decode(new(any)) != io.EOF || in.Issuer != p.issuer || strings.TrimSpace(in.Subject) == "" || len(in.Subject) > 512 {
		fail(w, 400, "Invalid identity")
		return
	}
	family, err := p.catalog.PortalFamily(r.Context(), in.Issuer, in.Subject)
	if errors.Is(err, domain.ErrUnauthorized) {
		jsonResponse(w, 200, map[string]any{"connected": false})
		return
	}
	if err != nil || family <= 0 {
		fail(w, 503, "Subscription summary unavailable")
		return
	}
	// Never forward the reader key or browser credentials to Core.
	internal := r.Clone(r.Context())
	internal.Header = make(http.Header)
	body, _ := json.Marshal(map[string]int64{"familyId": family})
	response, status, err := g.rpc(internal, http.MethodPost, httpapi.SubscriptionSummaryPath, body)
	var sub domain.FamilySubscription
	if err != nil || status != 200 || json.Unmarshal(response, &sub) != nil || sub.FamilyID != family {
		fail(w, 503, "Subscription summary unavailable")
		return
	}
	paid, err := p.catalog.Paid(r.Context(), family)
	if err != nil {
		fail(w, 503, "Subscription summary unavailable")
		return
	}
	if r.URL.Path == PortalBillingPath {
		historyReader, ok := p.catalog.(interface {
			HasPaymentHistory(context.Context, int64) (bool, error)
		})
		if !ok {
			fail(w, 503, "Billing context unavailable")
			return
		}
		history, err := historyReader.HasPaymentHistory(r.Context(), family)
		if err != nil {
			fail(w, 503, "Billing context unavailable")
			return
		}
		eligibility := "eligible"
		if sub.Status != "active" {
			eligibility = "suspended"
		} else if sub.AccessUntil == nil {
			eligibility = "lifetime"
		} else if history {
			eligibility = "payment_history"
		} else if sub.Access || paid {
			eligibility = "existing_access"
		}
		jsonResponse(w, 200, map[string]any{"connected": true, "subjectRef": "familyquest:" + strconv.FormatInt(family, 10), "name": sub.Name, "eligibility": eligibility, "billingManaged": sub.BillingManaged})
		return
	}
	jsonResponse(w, 200, map[string]any{"connected": true, "subscription": PortalSubscription{
		Name: sub.Name, Status: sub.Status, Plan: sub.Plan, ChildLimit: sub.ChildLimit,
		ActiveChildren: sub.ActiveChildren, AccessUntil: sub.AccessUntil, Paid: paid, Access: sub.Access, ChildrenUnlimited: sub.ChildrenUnlimited, BillingManaged: sub.BillingManaged,
	}})
}
