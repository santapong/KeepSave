package service

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"testing"
)

func TestDisabledBuilderCannotDispatch(t *testing.T) {
	b := NewDisabledMCPBuilderService()
	id := uuid.New()
	for _, call := range []func() error{func() error { return b.EnqueueBuild(id) }, func() error { return b.EnqueueRebuild(id) }, func() error { return b.BuildServerCtx(context.Background(), id) }, func() error { return b.RebuildServerCtx(context.Background(), id) }} {
		if err := call(); !errors.Is(err, ErrLocalMCPDisabled) {
			t.Fatal("disabled builder entered repository/process path", err)
		}
	}
}
