package lovit

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/lobov/familyquest/backend/internal/domain"
)

type BindingStore interface {
	LOVITBindings(context.Context, int64) ([]domain.LOVITBinding, error)
	ApplyLOVIT(context.Context, int64, domain.LOVITEntitlement, string) error
}
type Checker interface {
	Check(context.Context, string, string) (Entitlement, error)
}

// SyncPass reads only explicitly bound families, with four bounded readers.
// Errors do not produce a fabricated entitlement or prevent other families syncing.
func SyncPass(ctx context.Context, store BindingStore, client Checker, operator string) (int64, error) {
	var applied, failed atomic.Int64
	var after int64
	for {
		bindings, err := store.LOVITBindings(ctx, after)
		if err != nil {
			return applied.Load(), fmt.Errorf("binding list unavailable")
		}
		if len(bindings) == 0 {
			break
		}
		// Check monotonic paging before starting any side effect.
		for _, b := range bindings {
			if b.FamilyID <= after || b.SubscriptionID == "" {
				return applied.Load(), fmt.Errorf("invalid binding page")
			}
			after = b.FamilyID
		}
		jobs := make(chan domain.LOVITBinding)
		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for b := range jobs {
					rights, e := client.Check(ctx, fmt.Sprintf("familyquest:%d", b.FamilyID), b.SubscriptionID)
					if e == nil {
						e = store.ApplyLOVIT(ctx, b.FamilyID, rights, operator)
					}
					if e != nil {
						failed.Add(1)
					} else {
						applied.Add(1)
					}
				}
			}()
		}
		for _, b := range bindings {
			select {
			case jobs <- b:
			case <-ctx.Done():
				close(jobs)
				wg.Wait()
				return applied.Load(), ctx.Err()
			}
		}
		close(jobs)
		wg.Wait()
	}
	if failed.Load() > 0 {
		return applied.Load(), fmt.Errorf("%d bindings could not be synchronized", failed.Load())
	}
	return applied.Load(), nil
}
