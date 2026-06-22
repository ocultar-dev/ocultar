package audit

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func readEvents(t *testing.T, path string) []Event {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e Event
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("unmarshal event: %v", err)
		}
		events = append(events, e)
	}
	return events
}

func TestRotation_TriggersAtSizeThreshold(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := NewImmutableLogger(path)
	if err != nil {
		t.Fatalf("NewImmutableLogger: %v", err)
	}
	defer l.Close()

	l.SetRotation(300, 0) // tiny threshold; archive purge disabled for this test

	for i := 0; i < 10; i++ {
		if err := l.Log("tester", "TEST_EVENT", "resource", "ALLOW", fmt.Sprintf("event %d", i)); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}

	matches, err := filepath.Glob(path + ".*.archived")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("expected at least one archived file after exceeding size threshold, found none")
	}
}

func TestRotation_ChainContinuityAcrossBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := NewImmutableLogger(path)
	if err != nil {
		t.Fatalf("NewImmutableLogger: %v", err)
	}
	defer l.Close()

	l.SetRotation(300, 0)

	for i := 0; i < 10; i++ {
		if err := l.Log("tester", "TEST_EVENT", "resource", "ALLOW", fmt.Sprintf("event %d", i)); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}

	matches, err := filepath.Glob(path + ".*.archived")
	if err != nil || len(matches) == 0 {
		t.Fatalf("expected an archived file, got matches=%v err=%v", matches, err)
	}
	// Lexical sort == chronological order (fixed-width zero-padded timestamp
	// in the filename); the active file's first event only continues the
	// chain from the most recent rotation, not necessarily the first one.
	archivePath := matches[len(matches)-1]

	archiveEvents := readEvents(t, archivePath)
	if len(archiveEvents) == 0 {
		t.Fatal("archive file has no events")
	}
	rotateEvent := archiveEvents[len(archiveEvents)-1]
	if rotateEvent.Action != "CHECKPOINT_ROTATE" {
		t.Fatalf("want last archive event to be CHECKPOINT_ROTATE, got %q", rotateEvent.Action)
	}

	// Recompute the rotation checkpoint's hash the same way logLocked does,
	// and confirm the new file's first event continues the chain from it.
	rotatePayload := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s",
		rotateEvent.Timestamp, rotateEvent.Actor, rotateEvent.Action, rotateEvent.Resource,
		rotateEvent.Status, rotateEvent.Details, rotateEvent.PrevHash)
	h := sha256.Sum256([]byte(rotatePayload))
	wantPrevHash := hex.EncodeToString(h[:])

	activeEvents := readEvents(t, path)
	if len(activeEvents) == 0 {
		t.Fatal("active file has no events")
	}
	continueEvent := activeEvents[0]
	if continueEvent.Action != "CHECKPOINT_CONTINUE" {
		t.Fatalf("want first active event to be CHECKPOINT_CONTINUE, got %q", continueEvent.Action)
	}
	if continueEvent.PrevHash != wantPrevHash {
		t.Errorf("chain broken across rotation boundary: want PrevHash %q, got %q", wantPrevHash, continueEvent.PrevHash)
	}
}

func TestRotation_ArchivePurgeByAge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := NewImmutableLogger(path)
	if err != nil {
		t.Fatalf("NewImmutableLogger: %v", err)
	}
	defer l.Close()

	l.SetRotation(300, 24*time.Hour)

	for i := 0; i < 5; i++ {
		if err := l.Log("tester", "TEST_EVENT", "resource", "ALLOW", fmt.Sprintf("event %d", i)); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}

	before, err := filepath.Glob(path + ".*.archived")
	if err != nil || len(before) == 0 {
		t.Fatalf("expected at least one archived file before backdating, got %v err=%v", before, err)
	}

	old := time.Now().Add(-48 * time.Hour)
	for _, m := range before {
		if err := os.Chtimes(m, old, old); err != nil {
			t.Fatalf("Chtimes(%s): %v", m, err)
		}
	}

	// Trigger more rotations so purgeOldArchivesLocked runs again and sees
	// the now-stale archives.
	for i := 0; i < 5; i++ {
		if err := l.Log("tester", "TEST_EVENT", "resource", "ALLOW", fmt.Sprintf("event-2 %d", i)); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}

	for _, m := range before {
		if _, err := os.Stat(m); !os.IsNotExist(err) {
			t.Errorf("want backdated archive %s to be purged, but it still exists (stat err=%v)", m, err)
		}
	}

	after, err := filepath.Glob(path + ".*.archived")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(after) == 0 {
		t.Error("want fresh (non-backdated) archives created by the second loop to remain")
	}
}
