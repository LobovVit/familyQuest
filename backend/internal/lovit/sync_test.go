package lovit

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/lobov/familyquest/backend/internal/domain"
)

type fakeBindings struct {
	mu      sync.Mutex
	applied []int64
}

func (f *fakeBindings) LOVITBindings(_ context.Context, after int64) ([]domain.LOVITBinding, error) {
	if after > 0 {
		return nil, nil
	}
	return []domain.LOVITBinding{{FamilyID: 7, SubscriptionID: "sub_a"}, {FamilyID: 8, SubscriptionID: "sub_b"}}, nil
}
func (f *fakeBindings) ApplyLOVIT(_ context.Context, id int64, _ domain.LOVITEntitlement, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.applied = append(f.applied, id)
	return nil
}

type fakeChecks struct{}

func (fakeChecks) Check(_ context.Context, subject, subscription string) (Entitlement, error) {
	if subject == "familyquest:7" && subscription == "sub_a" {
		return Entitlement{}, nil
	}
	return Entitlement{}, errors.New("offline")
}
func TestSyncPassNeverInventsRightsAfterFailure(t *testing.T) {
	store := &fakeBindings{}
	count, err := SyncPass(context.Background(), store, fakeChecks{}, "test")
	if count != 1 || err == nil || len(store.applied) != 1 || store.applied[0] != 7 {
		t.Fatal(count, err, store.applied)
	}
}
