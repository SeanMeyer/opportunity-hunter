package pipeline_test

import (
	"testing"

	"github.com/seanmeyer/opportunity-hunter/pipeline"
)

func TestRunGuard(t *testing.T) {
	g := pipeline.NewRunGuard()

	if !g.TryAcquire("comedy") {
		t.Fatal("should acquire comedy")
	}

	if g.TryAcquire("comedy") {
		t.Fatal("should not acquire comedy twice")
	}

	if g.TryAcquire("powder") {
		t.Fatal("should not acquire powder while comedy is running")
	}

	g.Release("comedy")

	if !g.TryAcquire("powder") {
		t.Fatal("should acquire powder after comedy released")
	}

	if g.Running() != "powder" {
		t.Errorf("expected running=powder, got %q", g.Running())
	}

	g.Release("powder")

	if g.Running() != "" {
		t.Errorf("expected empty, got %q", g.Running())
	}
}
