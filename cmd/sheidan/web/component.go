package main

import "github.com/gopherjs/gopherjs/js"

// Component binds a render function to a container element. The render
// function returns the virtual children of the container. update runs the
// render function to build a new virtual tree and patches the
// container's real children with the minimal difference from the
// previous render, so only the changed parts touch the real DOM.
type Component struct {
	doc       *js.Object
	container *js.Object
	render    func() []*VNode
	prev      []*VNode
	sched     *Scheduler
}

// NewComponent creates a component that renders into container. The
// component is not rendered until update is called, directly or through
// the scheduler.
func NewComponent(sched *Scheduler, doc *js.Object, container *js.Object, render func() []*VNode) *Component {
	return &Component{doc: doc, container: container, render: render, sched: sched}
}

// update re-renders and patches the container's children. The scheduler
// calls it on the next tick, batched; the initial mount calls it
// directly, so the first render is not delayed by a tick.
func (c *Component) update() {
	next := c.render()
	patchChildren(c.doc, c.prev, next, c.container)
	c.prev = next
}

// schedule queues an update on the next tick of the event loop.
func (c *Component) schedule() {
	c.sched.Schedule(c)
}
