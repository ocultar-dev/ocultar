package handlers

import (
	"testing"
	"time"
)

func TestRevealRateLimiter_BlocksAfterLimit(t *testing.T) {
	rl := newRevealRateLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		if !rl.allow("token-a") {
			t.Fatalf("request %d should be allowed within limit", i+1)
		}
	}
	if rl.allow("token-a") {
		t.Error("4th request should be blocked once the limit is reached")
	}

	// A different key has its own independent budget.
	if !rl.allow("token-b") {
		t.Error("a different key should not be affected by token-a's limit")
	}
}

func TestRevealRateLimiter_ResetsAfterWindow(t *testing.T) {
	rl := newRevealRateLimiter(1, 50*time.Millisecond)

	if !rl.allow("token-a") {
		t.Fatal("first request should be allowed")
	}
	if rl.allow("token-a") {
		t.Error("second request within the window should be blocked")
	}

	time.Sleep(60 * time.Millisecond)

	if !rl.allow("token-a") {
		t.Error("request after the window elapses should be allowed again")
	}
}
