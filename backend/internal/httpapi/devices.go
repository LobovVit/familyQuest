package httpapi

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/lobov/familyquest/backend/internal/domain"
)

const deviceCookie = "familyquest_device"

func cookieValue(r *http.Request) string {
	c, err := r.Cookie(deviceCookie)
	if err != nil {
		return ""
	}
	return c.Value
}
func setDeviceCookie(w http.ResponseWriter, r *http.Request, value string) {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	secure := host != "localhost" && host != "127.0.0.1" && host != "::1"
	c := &http.Cookie{Name: deviceCookie, Value: value, Path: "/api", HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode, MaxAge: int(domain.DeviceLifetime.Seconds()), Expires: time.Now().Add(domain.DeviceLifetime)}
	if value == "" {
		c.MaxAge = -1
		c.Expires = time.Unix(1, 0)
	}
	http.SetCookie(w, c)
}

// Custom header prevents form CSRF; Origin is checked independently of CORS.
func sameSiteRequest(r *http.Request) bool {
	if r.Header.Get("X-FamilyQuest") != "1" {
		return false
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") {
			return false
		}
	}
	return true
}
func (s *Server) requestPrincipal(w http.ResponseWriter, r *http.Request) (domain.Principal, error) {
	if r.Header.Get("Authorization") != "" {
		token, err := parseBearer(r.Header.Get("Authorization"))
		if err != nil {
			return domain.Principal{}, err
		}
		return s.store.ParseToken(r.Context(), token)
	}
	if !sameSiteRequest(r) {
		return domain.Principal{}, domain.ErrUnauthorized
	}
	secret := cookieValue(r)
	p, d, err := s.store.DeviceSession(r.Context(), secret)
	if err != nil {
		return domain.Principal{}, err
	}
	// Renew the browser cookie on session restoration, not on parallel data requests.
	if r.Method == http.MethodGet && r.URL.Path == "/api/session" {
		setDeviceCookie(w, r, secret)
	}
	return domain.Principal{ParticipantID: p.ID, Role: p.Role, SessionVersion: p.SessionVersion, DeviceID: d.ID}, nil
}
func (s *Server) deviceRoutes() {
	s.mux.HandleFunc("GET /api/session", func(w http.ResponseWriter, r *http.Request) {
		p, err := s.requestPrincipal(w, r)
		if err != nil {
			respond(w, nil, err)
			return
		}
		owner, err := s.store.ListParticipants(r.Context())
		if err != nil {
			respond(w, nil, err)
			return
		}
		for _, v := range owner {
			if v.ID == p.ParticipantID {
				token, _ := parseBearer(r.Header.Get("Authorization"))
				writeJSON(w, 200, map[string]any{"participant": v, "token": token, "remembered": p.DeviceID != "", "deviceId": p.DeviceID})
				return
			}
		}
		respond(w, nil, domain.ErrUnauthorized)
	})
	s.mux.HandleFunc("POST /api/session/logout", func(w http.ResponseWriter, r *http.Request) {
		if !sameSiteRequest(r) {
			respond(w, nil, domain.ErrForbidden)
			return
		}
		if err := s.store.ForgetDevice(r.Context(), cookieValue(r)); err != nil {
			respond(w, nil, err)
			return
		}
		setDeviceCookie(w, r, "")
		writeJSON(w, 200, map[string]string{"status": "signed-out"})
	})
	s.mux.Handle("POST /api/session/confirm", s.authorize(true, func(w http.ResponseWriter, r *http.Request) {
		if !sameSiteRequest(r) {
			respond(w, nil, domain.ErrForbidden)
			return
		}
		var body struct {
			PIN string `json:"pin"`
		}
		if err := decodeJSON(r, &body); err != nil {
			respond(w, nil, domain.ErrInvalidInput)
			return
		}
		proof, err := s.store.ConfirmParent(r.Context(), principal(r), body.PIN)
		respond(w, map[string]string{"proof": proof}, err)
	}))
	s.mux.Handle("GET /api/devices", s.authorize(true, func(w http.ResponseWriter, r *http.Request) {
		v, err := s.store.Devices(r.Context(), principal(r))
		respond(w, v, err)
	}))
	s.mux.Handle("DELETE /api/devices/{id}", s.authorize(true, func(w http.ResponseWriter, r *http.Request) {
		err := s.store.RemoveDevice(r.Context(), principal(r), r.PathValue("id"), r.Header.Get("X-FamilyQuest-Confirmation"))
		if err == nil && principal(r).DeviceID == r.PathValue("id") {
			setDeviceCookie(w, r, "")
		}
		respond(w, map[string]string{"status": "revoked"}, err)
	}))
}

// Keep the actor in context for all adapters, regardless of credential type.
func withPrincipal(r *http.Request, p domain.Principal) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), principalKey{}, p))
}
