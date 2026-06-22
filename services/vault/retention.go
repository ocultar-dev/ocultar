package vault

import (
	"context"
	"time"
)

// RunRetentionLoop periodically purges vault rows older than retentionWindow,
// enforcing GDPR Art. 5(1)(e) storage limitation. It blocks until ctx is
// cancelled, so callers should invoke it in a goroutine.
//
// onSweep, if non-nil, is called after every sweep (even ones that purge zero
// rows or fail) so the caller can record an audit trail of enforcement, not
// just of the deletions themselves. RunRetentionLoop takes a callback rather
// than a concrete audit logger type so this package never depends on
// services/refinery (which already depends on services/vault).
func RunRetentionLoop(ctx context.Context, p Provider, sweepInterval, retentionWindow time.Duration, onSweep func(deleted int64, err error)) {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := p.PurgeExpiredTokens(time.Now().Add(-retentionWindow))
			if onSweep != nil {
				onSweep(n, err)
			}
		}
	}
}
