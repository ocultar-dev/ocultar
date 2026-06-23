//go:build ignore

package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/ocultar-dev/ocultar/pkg/config"
	"github.com/ocultar-dev/ocultar/vault"

	_ "github.com/lib/pq"
)

const (
	dsn         = "postgres://postgres:secret@localhost:5432/postgres?sslmode=disable"
	numNodes    = 5
	concurrency = 50
	numRecords  = 1000
)

func generateRandomHash() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// fatalf logs an error at Error level and exits — slog has no built-in
// fatal-and-exit, so this restores the log.Fatalf call sites it replaces.
func fatalf(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}

func main() {
	slog.Info("starting HA cluster verification & benchmark")

	nodes := make([]vault.Provider, numNodes)
	var nodesMu sync.RWMutex
	var err error

	// Truncate the table at start to ensure clean benchmark metrics
	db, err := sql.Open("postgres", dsn)
	if err == nil {
		db.Exec("TRUNCATE TABLE vault;")
		db.Close()
	}

	for i := 0; i < numNodes; i++ {
		nodes[i], err = vault.New(config.Settings{VaultBackend: "postgres", PostgresDSN: dsn}, "")
		if err != nil {
			fatalf("failed to initialize node", "node", i, "error", err)
		}
		slog.Info("node initialized successfully", "node", i)
	}

	// Prepare records
	slog.Info("generating unique PII hashes", "count", numRecords)
	records := make([]string, numRecords)
	for i := 0; i < numRecords; i++ {
		records[i] = generateRandomHash()
	}

	start := time.Now()
	var wg sync.WaitGroup

	slog.Info("launching concurrent workers", "workers", concurrency, "nodes", numNodes)

	// Benchmark Concurrent Writes
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			nodeIdx := workerID % numNodes

			for j := 0; j < numRecords; j++ {
				hash := records[j]
				token := fmt.Sprintf("[TOKEN_%s]", hash[:8])
				encPII := "ENCRYPTED_" + hash

				nodesMu.RLock()
				node := nodes[nodeIdx]
				nodesMu.RUnlock()

				// If node is nil or closed, we will naturally error here and log it.
				// But we continue to simulate an external retry logic.
				if node == nil {
					continue
				}

				// In a real cluster, nodes race to insert. Safe due to ON CONFLICT DO NOTHING.
				_, err := node.StoreToken(hash, token, encPII)
				if err != nil {
					slog.Error("worker failed insertion", "worker", workerID, "node", nodeIdx, "error", err)
					time.Sleep(10 * time.Millisecond) // backoff on failure
				}
			}
		}(i)
	}

	// Simulate node failure and rolling restart
	go func() {
		time.Sleep(1 * time.Second)
		slog.Warn("simulating node failure: dropping cluster node 1")

		nodesMu.Lock()
		nodes[1].Close()
		nodes[1] = nil
		nodesMu.Unlock()

		time.Sleep(500 * time.Millisecond)
		slog.Info("simulating rolling restart: recovering cluster node 1")

		newNode, err := vault.New(config.Settings{VaultBackend: "postgres", PostgresDSN: dsn}, "")

		nodesMu.Lock()
		nodes[1] = newNode
		nodesMu.Unlock()

		if err != nil {
			slog.Error("rolling restart failed for node 1", "error", err)
		} else {
			slog.Info("node 1 recovered gracefully")
		}
	}()

	wg.Wait()
	duration := time.Since(start)

	slog.Info("HA benchmark results", "total_duration", duration, "ops_per_second", float64(numRecords*concurrency)/duration.Seconds())

	// Clean shutdown check
	for i, n := range nodes {
		if n == nil {
			slog.Warn("node is nil, skipping close", "node", i)
			continue
		}
		if err := n.Close(); err != nil {
			slog.Warn("node failed to cleanly shut down", "node", i, "error", err)
		} else {
			slog.Info("node gracefully detached from cluster", "node", i)
		}
	}

	// Final validation check with a fresh connection
	validator, _ := vault.New(config.Settings{VaultBackend: "postgres", PostgresDSN: dsn}, "")
	defer validator.Close()

	count := validator.CountAll()
	if count == int64(numRecords) {
		slog.Info("vault idempotency verified", "expected", numRecords, "got", count)
	} else {
		fatalf("cluster race condition detected", "expected", numRecords, "got", count)
	}

	slog.Info("Postgres HA stateless scaling validated successfully in multi-node topology")
}
