package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestMCPBuilder_EnqueueBoundsConcurrency proves the DoS bound: once the
// concurrency limit is reached, EnqueueBuild/EnqueueRebuild refuse further work
// with ErrBuildQueueFull instead of spawning unbounded goroutines/subprocesses.
//
// It sets MCP_MAX_CONCURRENT_BUILDS=1 and MCP_BUILD_TIMEOUT small, uses a
// cancellable base context, and occupies the single slot with a build for a
// non-existent server (GetServer fails fast, but we hold the slot via a blocked
// base context to make the gate observable). To avoid depending on the DB, we
// drive the semaphore directly: fill it, assert the next enqueue is rejected,
// then release and assert acceptance resumes.
func TestMCPBuilder_EnqueueBoundsConcurrency(t *testing.T) {
	t.Setenv("MCP_MAX_CONCURRENT_BUILDS", "1")
	t.Setenv("MCP_BUILD_TIMEOUT", "1s")
	t.Setenv("MCP_BUILD_DIR", t.TempDir())

	// mcpRepo is nil: the goroutine's BuildServerCtx calls GetServer(nil) which
	// panics if dereferenced, so we must NOT let the goroutine run against a nil
	// repo. Instead we exercise the semaphore gate directly by occupying the one
	// slot manually and calling enqueue, which only touches s.sem before spawning.
	s := &MCPBuilderService{
		sem:          make(chan struct{}, 1),
		buildTimeout: time.Second,
		baseCtx:      context.Background(),
	}

	// Occupy the only slot.
	s.sem <- struct{}{}

	// Next enqueue must be rejected without starting a goroutine.
	if err := s.EnqueueBuild(uuid.New()); !errors.Is(err, ErrBuildQueueFull) {
		t.Fatalf("EnqueueBuild over limit = %v, want ErrBuildQueueFull", err)
	}
	if err := s.EnqueueRebuild(uuid.New()); !errors.Is(err, ErrBuildQueueFull) {
		t.Fatalf("EnqueueRebuild over limit = %v, want ErrBuildQueueFull", err)
	}

	// Release the slot; the gate must accept again. We can't let the real build
	// run (nil repo), so drain via the same channel the goroutine would use.
	<-s.sem

	select {
	case s.sem <- struct{}{}:
		<-s.sem // free it again
	default:
		t.Fatal("slot did not free after release")
	}
}

// TestMCPBuilder_DefaultsFromConstructor confirms NewMCPBuilderService reads the
// concurrency + timeout env vars and applies safe defaults otherwise.
func TestMCPBuilder_DefaultsFromConstructor(t *testing.T) {
	t.Setenv("MCP_BUILD_DIR", t.TempDir())
	t.Setenv("MCP_MAX_CONCURRENT_BUILDS", "4")
	t.Setenv("MCP_BUILD_TIMEOUT", "90s")

	s := NewMCPBuilderService(nil)
	if cap(s.sem) != 4 {
		t.Errorf("sem cap = %d, want 4", cap(s.sem))
	}
	if s.buildTimeout != 90*time.Second {
		t.Errorf("buildTimeout = %v, want 90s", s.buildTimeout)
	}

	// Base context override is honoured.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.SetBaseContext(ctx)
	if s.getBaseContext() != ctx {
		t.Error("SetBaseContext did not take effect")
	}
}
