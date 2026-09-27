package main

import (
	"testing"

	"github.com/gopherjs/gopherjs/js"
)

// TestNullUndefinedSemantics pins down the js package's null and
// undefined semantics that the fake DOM relies on. A *js.Object holding
// JavaScript's null compares equal to nil, but a property that was never
// set is JavaScript's undefined, which does not compare equal to nil.
// The fake DOM therefore initializes every pointer property to an
// explicit null instead of leaving it unset.
func TestNullUndefinedSemantics(t *testing.T) {
	obj := js.Global.Get("Object").New()

	// An explicit null round-trips through Set and Get.
	obj.Set("setNull", (*js.Object)(nil))
	if got := obj.Get("setNull"); got != nil {
		t.Errorf("Get on set null = %v; want nil", got)
	}

	// A never-set property is undefined, not null.
	if got := obj.Get("unset"); got == nil {
		t.Error("Get on unset property = nil; want non-nil (undefined)")
	}
	if got := obj.Get("unset"); got != js.Undefined {
		t.Errorf("Get on unset property = %v; want js.Undefined", got)
	}

	// A set object property round-trips as the same object: two Gets of
	// the same property compare equal, so identity checks on *js.Object
	// values work.
	el := js.Global.Get("Object").New()
	obj.Set("el", el)
	if a, b := obj.Get("el"), obj.Get("el"); a != b {
		t.Error("two Gets of the same property are not equal; want the same object")
	}
	if a, b := obj.Get("el"), el; a != b {
		t.Error("Get of a set property is not equal to the original; want the same object")
	}
}
