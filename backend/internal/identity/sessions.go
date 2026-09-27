package identity

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Sessions is a single-instance, fail-closed session registry. Restart requires SSO again.
// Sessions хранит сессии одного экземпляра; перезапуск требует повторного SSO.
type Sessions struct {
	file    string
	mu      sync.Mutex
	active  map[string]Session
	revoked map[string]time.Time
}
type Session struct {
	Subject Subject
	Scope   string
	Expires time.Time
}

func NewSessions() *Sessions {
	return &Sessions{active: map[string]Session{}, revoked: map[string]time.Time{}}
}
func (s *Sessions) sweep(now time.Time) {
	for k, v := range s.active {
		if !now.Before(v.Expires) {
			delete(s.active, k)
		}
	}
	for k, v := range s.revoked {
		if !now.Before(v) {
			delete(s.revoked, k)
		}
	}
}
func (s *Sessions) Add(sub Subject, scope string) (string, error) {
	return s.AddFor(sub, scope, 12*time.Hour)
}
func (s *Sessions) AddFor(sub Subject, scope string, ttl time.Duration) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.sweep(now)
	if sub.ID == "" || sub.SessionID == "" || len(s.active) >= 10000 || len(s.revoked) >= 10000 {
		return "", errors.New("SSO session unavailable")
	}
	if _, ok := s.revoked["sid:"+sub.SessionID]; ok {
		return "", errors.New("SSO session revoked")
	}
	if _, ok := s.revoked["sub:"+sub.ID]; ok {
		return "", errors.New("SSO subject revoked")
	}
	id := random()
	s.active[id] = Session{sub, scope, now.Add(ttl)}
	if err := s.save(); err != nil {
		delete(s.active, id)
		return "", err
	}
	return id, nil
}
func (s *Sessions) Get(id string) (Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.active[id]
	if !ok || !time.Now().Before(v.Expires) {
		delete(s.active, id)
		return Session{}, false
	}
	return v, true
}
func (s *Sessions) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.active, id)
	s.persistOrClear()
}
func (s *Sessions) Revoke(sub Subject) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistOrClear()
	now := time.Now()
	s.sweep(now)
	// At capacity invalidate all local sessions rather than silently dropping revocations.
	// При переполнении отзываем локальные сессии, а не теряем событие отзыва.
	if len(s.revoked) >= 10000 {
		s.active = map[string]Session{}
		return
	}
	key := "sid:" + sub.SessionID
	if sub.SessionID == "" {
		key = "sub:" + sub.ID
	}
	s.revoked[key] = now.Add(30 * 24 * time.Hour)
	for k, v := range s.active {
		if v.Subject.Issuer == sub.Issuer && ((sub.SessionID != "" && v.Subject.SessionID == sub.SessionID) || (sub.SessionID == "" && v.Subject.ID == sub.ID)) {
			delete(s.active, k)
		}
	}
}

// NewPersistentSessions loads a single-instance session registry from a private volume.
// NewPersistentSessions восстанавливает сессии одного экземпляра из закрытого тома.
func NewPersistentSessions(path string) (*Sessions, error) {
	s := NewSessions()
	s.file = path
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil {
		var disk struct {
			Active  map[string]Session
			Revoked map[string]time.Time
		}
		if err = json.Unmarshal(data, &disk); err != nil {
			return nil, err
		}
		if disk.Active != nil {
			s.active = disk.Active
		}
		if disk.Revoked != nil {
			s.revoked = disk.Revoked
		}
	}
	s.sweep(time.Now())
	return s, s.save()
}
func (s *Sessions) save() error {
	if s.file == "" {
		return nil
	}
	b, err := json.Marshal(struct {
		Active  map[string]Session
		Revoked map[string]time.Time
	}{s.active, s.revoked})
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.file), ".sessions-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), s.file)
}
func (s *Sessions) persistOrClear() {
	if s.save() != nil {
		panic("cannot persist SSO revocation")
	}
}
