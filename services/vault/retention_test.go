package vault

// retention_test.go — tests for TTL purge of vault PII rows (retention.go,
// PurgeExpiredTokens, DeleteToken). Unlike the other *_test.go files in this
// package (which are black-box, package vault_test), this file is white-box
// (package vault) so it can seed rows with backdated created_at timestamps
// directly via SQL — something the public Provider interface intentionally
// does not expose.

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func openTestDuckDB(t *testing.T) *duckdbProvider {
	t.Helper()
	vaultPath := filepath.Join(t.TempDir(), "retention.db")
	p, err := newDuckDBProvider(vaultPath)
	if err != nil {
		t.Fatalf("newDuckDBProvider: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}

// seedRowAt inserts a vault row with an explicit created_at, bypassing
// StoreToken (which always stamps current_timestamp).
func seedRowAt(t *testing.T, p *duckdbProvider, hash, token, encrypted string, createdAt time.Time) {
	t.Helper()
	_, err := p.db.Exec(
		`INSERT INTO vault (pii_hash, token, encrypted_pii, created_at) VALUES (?, ?, ?, ?)`,
		hash, token, encrypted, createdAt,
	)
	if err != nil {
		t.Fatalf("seedRowAt: %v", err)
	}
	p.count.Add(1)
}

func TestPurgeExpiredTokens_DeletesOnlyOlderThanCutoff(t *testing.T) {
	p := openTestDuckDB(t)

	now := time.Now().UTC()
	seedRowAt(t, p, "hash-old-1", "[EMAIL_old1]", "enc1", now.Add(-100*24*time.Hour))
	seedRowAt(t, p, "hash-old-2", "[EMAIL_old2]", "enc2", now.Add(-91*24*time.Hour))
	seedRowAt(t, p, "hash-new-1", "[EMAIL_new1]", "enc3", now.Add(-10*24*time.Hour))
	seedRowAt(t, p, "hash-new-2", "[EMAIL_new2]", "enc4", now)

	cutoff := now.Add(-90 * 24 * time.Hour)
	deleted, err := p.PurgeExpiredTokens(cutoff)
	if err != nil {
		t.Fatalf("PurgeExpiredTokens: %v", err)
	}
	if deleted != 2 {
		t.Errorf("want 2 rows deleted, got %d", deleted)
	}

	if _, found := p.GetToken("hash-old-1"); found {
		t.Errorf("hash-old-1 should have been purged")
	}
	if _, found := p.GetToken("hash-new-1"); !found {
		t.Errorf("hash-new-1 should NOT have been purged")
	}
	if _, found := p.GetToken("hash-new-2"); !found {
		t.Errorf("hash-new-2 should NOT have been purged")
	}

	if got := p.CountAll(); got != 2 {
		t.Errorf("want CountAll()=2 after purge, got %d", got)
	}
}

func TestPurgeExpiredTokens_NeverTouchesEntityRegistry(t *testing.T) {
	p := openTestDuckDB(t)

	tok, err := p.RegisterEntity("PERSON", "Old Person", []string{"Old"})
	if err != nil {
		t.Fatalf("RegisterEntity: %v", err)
	}

	// Backdate a vault row far enough to guarantee it's purged, to prove the
	// purge actually ran, then confirm the Entity Registry is untouched.
	seedRowAt(t, p, "hash-old", "[EMAIL_old]", "enc", time.Now().Add(-365*24*time.Hour))

	deleted, err := p.PurgeExpiredTokens(time.Now().Add(-90 * 24 * time.Hour))
	if err != nil {
		t.Fatalf("PurgeExpiredTokens: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("want 1 row deleted, got %d", deleted)
	}

	if _, found := p.GetEntityByToken(tok); !found {
		t.Errorf("Entity Registry entry %s was deleted by vault TTL purge — must never happen", tok)
	}
}

func TestDeleteToken_RemovesRowAndDecrementsCount(t *testing.T) {
	p := openTestDuckDB(t)

	if _, err := p.StoreToken("hash-1", "[EMAIL_a1b2]", "enc1"); err != nil {
		t.Fatalf("StoreToken: %v", err)
	}
	if got := p.CountAll(); got != 1 {
		t.Fatalf("want CountAll()=1 before delete, got %d", got)
	}

	deleted, err := p.DeleteToken("[EMAIL_a1b2]")
	if err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}
	if !deleted {
		t.Errorf("want deleted=true for an existing token")
	}
	if got := p.CountAll(); got != 0 {
		t.Errorf("want CountAll()=0 after delete, got %d", got)
	}
	if _, found := p.GetEncryptedByToken("[EMAIL_a1b2]"); found {
		t.Errorf("token should no longer be retrievable after DeleteToken")
	}
}

func TestDeleteToken_NotFoundReturnsFalseNoError(t *testing.T) {
	p := openTestDuckDB(t)

	deleted, err := p.DeleteToken("[EMAIL_does_not_exist]")
	if err != nil {
		t.Fatalf("DeleteToken: unexpected error %v", err)
	}
	if deleted {
		t.Errorf("want deleted=false for a non-existent token")
	}
}

func TestRunRetentionLoop_FiresOnSweepCallback(t *testing.T) {
	p := openTestDuckDB(t)

	seedRowAt(t, p, "hash-old", "[EMAIL_old]", "enc", time.Now().Add(-365*24*time.Hour))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type sweepResult struct {
		deleted int64
		err     error
	}
	results := make(chan sweepResult, 4)

	go RunRetentionLoop(ctx, p, 10*time.Millisecond, 90*24*time.Hour, func(deleted int64, err error) {
		results <- sweepResult{deleted, err}
	})

	select {
	case r := <-results:
		if r.err != nil {
			t.Fatalf("sweep callback got error: %v", r.err)
		}
		if r.deleted != 1 {
			t.Errorf("want first sweep to purge 1 row, got %d", r.deleted)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for retention sweep callback")
	}
}
