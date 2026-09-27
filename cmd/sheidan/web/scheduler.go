package main

import "github.com/gopherjs/gopherjs/js"

// Scheduler batches component updates onto the next tick of the event
// loop. Scheduling the same component repeatedly within one tick marks it
// dirty once, so a burst of mutations costs a single render and patch per
// component instead of one per mutation.
type Scheduler struct {
	tick   func(fn func())
	dirty  map[*Component]struct{}
	queued bool
}

// NewScheduler creates a scheduler that flushes on the next tick of the
// event loop.
func NewScheduler() *Scheduler {
	return &Scheduler{tick: nextTick, dirty: map[*Component]struct{}{}}
}

// Schedule marks c dirty and queues a flush on the next tick. Calling it
// again for the same component before the flush re-renders c once.
func (s *Scheduler) Schedule(c *Component) {
	s.dirty[c] = struct{}{}
	if s.queued {
		return
	}
	s.queued = true
	s.tick(func() { s.flush() })
}

// flush re-renders every dirty component exactly once and clears the
// dirty set. The tick callback runs it; tests call it directly.
func (s *Scheduler) flush() {
	s.queued = false
	for c := range s.dirty {
		delete(s.dirty, c)
		c.update()
	}
}

// nextTick runs fn on the next tick of the event loop, through a Promise
// microtask. A microtask runs after the current task and its queued
// microtasks finish, so every mutation made in one task is batched into
// one flush.
func nextTick(fn func()) {
	js.Global.Get("Promise").Call("resolve").Call("then", js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		fn()
		return nil
	}))
}
