package audit

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Event represents a single entry in the immutable audit log.
type Event struct {
	Timestamp string `json:"timestamp"`
	Actor     string `json:"actor"`
	Action    string `json:"action"`
	Resource  string `json:"resource"`
	Status    string `json:"status"`
	Details   string `json:"details,omitempty"`
	PrevHash  string `json:"prev_hash"`
	Signature string `json:"signature"`
}

// ImmutableLogger handles cryptographically signed append-only logging
// to satisfy NIS2 and GDPR Article 30 requirements.
type ImmutableLogger struct {
	mu            sync.Mutex
	privateKey    ed25519.PrivateKey
	publicKey     ed25519.PublicKey
	path          string
	logFile       *os.File
	lastHash      string
	maxSizeBytes  int64
	archiveMaxAge time.Duration
}

// NewImmutableLogger initializes a logger on disk with an ephemeral Ed25519
// key pair. Signatures are verifiable within the current process session only.
// For cross-session verifiability use NewImmutableLoggerWithKey.
func NewImmutableLogger(filePath string) (*ImmutableLogger, error) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ed25519 keys: %v", err)
	}
	return newLogger(filePath, priv)
}

// NewImmutableLoggerWithKey initializes a logger with a caller-supplied
// private key. The same key must be retained to verify signatures after
// process restarts. privateKey must be a valid ed25519.PrivateKey (64 bytes).
func NewImmutableLoggerWithKey(filePath string, privateKey ed25519.PrivateKey) (*ImmutableLogger, error) {
	return newLogger(filePath, privateKey)
}

// LoadPrivateKeyFromHex decodes a hex-encoded 32-byte Ed25519 seed (as stored
// in OCU_AUDIT_PRIVATE_KEY) into a PrivateKey suitable for NewImmutableLoggerWithKey.
// Generate with: openssl rand -hex 32
func LoadPrivateKeyFromHex(seedHex string) (ed25519.PrivateKey, error) {
	seed, err := hex.DecodeString(seedHex)
	if err != nil {
		return nil, fmt.Errorf("OCU_AUDIT_PRIVATE_KEY is not valid hex: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("OCU_AUDIT_PRIVATE_KEY: want %d bytes, got %d", ed25519.SeedSize, len(seed))
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

func newLogger(filePath string, priv ed25519.PrivateKey) (*ImmutableLogger, error) {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log: %v", err)
	}
	return &ImmutableLogger{
		privateKey: priv,
		publicKey:  priv.Public().(ed25519.PublicKey),
		path:       filePath,
		logFile:    f,
		lastHash:   "0000000000000000000000000000000000000000000000000000000000000000",
	}, nil
}

// SetRotation configures size-based rotation of the active log file. Once it
// exceeds maxSizeBytes, a signed "CHECKPOINT_ROTATE" event is appended to the
// current file, which is then archived to a timestamped "<path>.<ts>.archived"
// file; a fresh file is opened at path and a "CHECKPOINT_CONTINUE" event is
// written as its first entry, carrying the in-memory hash chain forward so no
// signed event is ever mutated or deleted. Archived files older than
// archiveMaxAge are then removed. Leaving maxSizeBytes <= 0 (the default)
// disables rotation entirely, preserving existing behavior.
func (l *ImmutableLogger) SetRotation(maxSizeBytes int64, archiveMaxAge time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.maxSizeBytes = maxSizeBytes
	l.archiveMaxAge = archiveMaxAge
}

// Log records an event, chaining it to the previous hash and signing it.
func (l *ImmutableLogger) Log(actor, action, resource, status, details string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.logLocked(actor, action, resource, status, details); err != nil {
		return err
	}
	return l.rotateIfNeededLocked()
}

// logLocked writes a single signed, chained event to the currently open log
// file. Callers must hold l.mu.
func (l *ImmutableLogger) logLocked(actor, action, resource, status, details string) error {
	e := Event{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Actor:     actor,
		Action:    action,
		Resource:  resource,
		Status:    status,
		Details:   details,
		PrevHash:  l.lastHash,
	}

	// Payload to sign
	payload := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s",
		e.Timestamp, e.Actor, e.Action, e.Resource, e.Status, e.Details, e.PrevHash)

	sig := ed25519.Sign(l.privateKey, []byte(payload))
	e.Signature = hex.EncodeToString(sig)

	// Hash the payload for the next event in the chain
	h := sha256.Sum256([]byte(payload))
	l.lastHash = hex.EncodeToString(h[:])

	logData, err := json.Marshal(e)
	if err != nil {
		return err
	}

	if _, err := l.logFile.Write(append(logData, '\n')); err != nil {
		return fmt.Errorf("audit write failed: %v", err)
	}

	// Ensure it hits disk immediately (sync/fsync)
	return l.logFile.Sync()
}

// rotateIfNeededLocked archives the active log file once it exceeds
// l.maxSizeBytes, preserving hash-chain continuity across the rotation
// boundary via signed checkpoint events. Callers must hold l.mu.
func (l *ImmutableLogger) rotateIfNeededLocked() error {
	if l.maxSizeBytes <= 0 {
		return nil
	}

	info, err := l.logFile.Stat()
	if err != nil {
		return fmt.Errorf("audit rotation stat failed: %w", err)
	}
	if info.Size() < l.maxSizeBytes {
		return nil
	}

	// Sign the rotation boundary into the file about to be archived, so the
	// boundary itself is part of the verifiable chain.
	if err := l.logLocked("system", "CHECKPOINT_ROTATE", l.path, "ALLOW", "rotating audit log"); err != nil {
		return fmt.Errorf("audit rotation checkpoint failed: %w", err)
	}

	if err := l.logFile.Close(); err != nil {
		return fmt.Errorf("audit rotation close failed: %w", err)
	}

	archivePath := fmt.Sprintf("%s.%s.archived", l.path, time.Now().UTC().Format("20060102T150405.000000000"))
	if err := os.Rename(l.path, archivePath); err != nil {
		return fmt.Errorf("audit rotation rename failed: %w", err)
	}

	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("audit rotation reopen failed: %w", err)
	}
	l.logFile = f

	// First entry of the new file continues the chain: PrevHash is the
	// rotate-checkpoint's hash, already carried in l.lastHash.
	if err := l.logLocked("system", "CHECKPOINT_CONTINUE", archivePath, "ALLOW", "continuing audit log after rotation"); err != nil {
		return fmt.Errorf("audit rotation continuation failed: %w", err)
	}

	l.purgeOldArchivesLocked()
	return nil
}

// purgeOldArchivesLocked deletes archived log segments older than
// l.archiveMaxAge. Only ever removes whole, already-closed archive files —
// never an in-chain signed event in the active log. Callers must hold l.mu.
func (l *ImmutableLogger) purgeOldArchivesLocked() {
	if l.archiveMaxAge <= 0 {
		return
	}

	matches, err := filepath.Glob(l.path + ".*.archived")
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-l.archiveMaxAge)
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(m)
		}
	}
}

// Close gracefully closes the log file.
func (l *ImmutableLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.logFile.Close()
}

// PublicKey returns the hex string of the public key for verification.
func (l *ImmutableLogger) PublicKeyHex() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return hex.EncodeToString(l.publicKey)
}
