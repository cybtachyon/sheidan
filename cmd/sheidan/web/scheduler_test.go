package main

import "testing"

// countingComponent builds a component whose render function counts its
// invocations, so a test can verify how many times the scheduler
// re-rendered it.
func countingComponent(s *Scheduler, renders *int) *Component {
	doc := fakeDocument()
	container := fakeElement("div")
	return NewComponent(s, doc, container, func() []*VNode {
		*renders++
		return []*VNode{Text("x")}
	})
}

// deferredScheduler returns a scheduler whose tick defers the flush
// instead of running it, plus a function that runs every deferred
// flush. This lets a test drive the tick boundary explicitly.
func deferredScheduler() (*Scheduler, func()) {
	s := NewScheduler()
	var pending []func()
	s.tick = func(fn func()) {
		pending = append(pending, fn)
	}
	return s, func() {
		for len(pending) > 0 {
			fn := pending[0]
			pending = pending[1:]
			fn()
		}
	}
}

// TestSchedulerBatchesWithinTick verifies that scheduling the same
// component several times within one tick queues a single flush and
// re-renders the component once.
func TestSchedulerBatchesWithinTick(t *testing.T) {
	s, flush := deferredScheduler()
	var renders int
	c := countingComponent(s, &renders)

	c.schedule()
	c.schedule()
	c.schedule()

	if renders != 0 {
		t.Fatalf("renders before flush = %d; want 0", renders)
	}
	flush()
	if renders != 1 {
		t.Fatalf("renders after one flush = %d; want 1", renders)
	}
}

// TestSchedulerSeparateComponents verifies that two components scheduled
// in the same tick each re-render once.
func TestSchedulerSeparateComponents(t *testing.T) {
	s, flush := deferredScheduler()
	var a, b int
	ca := countingComponent(s, &a)
	cb := countingComponent(s, &b)

	ca.schedule()
	cb.schedule()
	ca.schedule()

	flush()
	if a != 1 || b != 1 {
		t.Fatalf("renders = a:%d b:%d; want 1:1", a, b)
	}
}

// TestSchedulerRetriggers verifies that a schedule after a flush queues a
// fresh tick, so a later burst of mutations is batched separately.
func TestSchedulerRetriggers(t *testing.T) {
	s, flush := deferredScheduler()
	var renders int
	c := countingComponent(s, &renders)

	c.schedule()
	flush()
	if renders != 1 {
		t.Fatalf("renders after first flush = %d; want 1", renders)
	}

	c.schedule()
	c.schedule()
	flush()
	if renders != 2 {
		t.Fatalf("renders after second flush = %d; want 2", renders)
	}
}
