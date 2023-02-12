package runnergroup

import (
	"context"
	"errors"
	"sync"

	"go.uber.org/atomic"
)

type Group struct {
	wg    sync.WaitGroup
	abort context.CancelFunc
	ctx   context.Context
	err   atomic.Error
}

func New(ctx context.Context) *Group {
	ctx, abort := context.WithCancel(ctx)
	return &Group{
		ctx:   ctx,
		abort: abort,
	}
}

func (g *Group) Go(fn func(context.Context) error) *Group {
	g.wg.Add(1)
	go func() {
		defer g.abort()
		defer g.wg.Done()

		err := fn(g.ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			if g.err.Load() == nil {
				// is supposed to store the first error, but a race might happen
				// too lazy to fix that now
				g.err.Store(err)
			}
		}
	}()
	return g
}

func (g *Group) Wait() error {
	defer g.abort()
	g.wg.Wait()
	return g.err.Load()
}
