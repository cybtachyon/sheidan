package main

import (
	"fmt"
	"testing"

	"github.com/gopherjs/gopherjs/js"
)

// listApp builds a minimal reactive app on the fake DOM: a store, a
// scheduler, and a component that renders the store's notes as a keyed
// list of rows, each with a delete button.
type listApp struct {
	doc     *js.Object
	store   *NoteStore
	sched   *Scheduler
	page    *Component
	renders int
}

// newListApp creates a listApp around the given scheduler.
func newListApp(sched *Scheduler) *listApp {
	a := &listApp{doc: fakeDocument(), store: NewNoteStore(), sched: sched}
	a.page = NewComponent(sched, a.doc, fakeElement("div"), a.render)
	a.store.Subscribe(a.page.schedule)
	return a
}

// render builds the virtual list from the store.
func (a *listApp) render() []*VNode {
	a.renders++
	var children []*VNode
	for _, note := range a.store.Notes() {
		children = append(children, a.row(note))
	}
	return children
}

// row builds one keyed list row with a delete button. It takes the note
// as a parameter, because Go 1.21 reuses the range variable across
// iterations, so a closure that captured it directly would remove the
// last note no matter which button was clicked.
func (a *listApp) row(note Note) *VNode {
	btn := El("button", nil, Text("delete"))
	WithEvents(btn, Event{Name: "click", Fn: js.MakeFunc(func(this *js.Object, args []*js.Object) any {
		a.store.Remove(note.ID)
		return nil
	})})
	return Keyed(El("li", nil, Text(note.Title), btn), fmt.Sprintf("%d", note.ID))
}

// container returns the listApp's real container element.
func (a *listApp) container() *js.Object { return a.page.container }

// TestComponentReactiveFlow verifies the reactive loop end to end: a
// store mutation schedules a re-render, the scheduler batches a burst of
// mutations into one render on the next tick, and the patch applies the
// result to the real DOM.
func TestComponentReactiveFlow(t *testing.T) {
	sched, flush := deferredScheduler()
	app := newListApp(sched)

	app.page.update()
	if app.renders != 1 {
		t.Fatalf("renders after initial update = %d; want 1", app.renders)
	}
	if got := fakeChildTags(app.container()); len(got) != 0 {
		t.Fatalf("children = %v; want none (empty store)", got)
	}

	// A burst of mutations within one tick re-renders once.
	app.store.Add(Note{ID: 1, Title: "a"})
	app.store.Add(Note{ID: 2, Title: "b"})
	if app.renders != 1 {
		t.Fatalf("renders after mutations = %d; want 1 (batched, not yet flushed)", app.renders)
	}

	flush()
	if app.renders != 2 {
		t.Fatalf("renders after flush = %d; want 2", app.renders)
	}
	if got := fakeChildTags(app.container()); len(got) != 2 || got[0] != "li" || got[1] != "li" {
		t.Fatalf("children = %v; want [li li]", got)
	}
	first := app.container().Get("children").Index(0)
	if got := fakeChildTags(first); len(got) != 2 || got[0] != "text:a" || got[1] != "button" {
		t.Fatalf("first row children = %v; want [text:a button]", got)
	}
}

// TestComponentDelete verifies that an event handler that mutates the
// store drives the component to remove the matching row on the next
// tick, reusing the surviving row's node.
func TestComponentDelete(t *testing.T) {
	sched, flush := deferredScheduler()
	app := newListApp(sched)
	app.store.SetAll([]Note{{ID: 1, Title: "a"}, {ID: 2, Title: "b"}})
	app.page.update()
	flush()

	container := app.container()
	survivor := container.Get("children").Index(1)
	rendersBefore := app.renders
	fakeClick(container.Get("children").Index(0).Get("children").Index(1))
	if app.renders != rendersBefore {
		t.Fatalf("renders after the click = %d; want %d (the delete must wait for the tick)", app.renders, rendersBefore)
	}
	flush()
	if app.renders != rendersBefore+1 {
		t.Fatalf("renders after the flush = %d; want %d", app.renders, rendersBefore+1)
	}

	if got := fakeChildTags(container); len(got) != 1 || got[0] != "li" {
		t.Fatalf("children = %v; want [li]", got)
	}
	if got := container.Get("children").Index(0); got != survivor {
		t.Error("the surviving row was not reused")
	}
	if got := container.Get("children").Index(0).Get("children").Index(0).Get("nodeValue").String(); got != "b" {
		t.Errorf("surviving text = %q; want %q", got, "b")
	}
}

// TestComponentUnchangedRerenderIsFree verifies that a re-render with no
// store change costs no DOM writes and no node creation.
func TestComponentUnchangedRerenderIsFree(t *testing.T) {
	sched, flush := deferredScheduler()
	app := newListApp(sched)
	app.store.SetAll([]Note{{ID: 1, Title: "a"}, {ID: 2, Title: "b"}})
	app.page.update()
	flush()
	fakeOps = 0
	fakeCreates = 0

	app.page.schedule()
	flush()

	if ops := fakeOpsCount(); ops != 0 {
		t.Errorf("ops = %d; want 0 for an unchanged re-render", ops)
	}
	if creates := fakeCreatesCount(); creates != 0 {
		t.Errorf("creates = %d; want 0 for an unchanged re-render", creates)
	}
}
