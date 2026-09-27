# Tasks Slice 14

Polish the reactive client.

## Tasks

1. Support multi-line note bodies. The body field currently uses a single-line
   input, so a body with newlines is hard to enter. Switch the body editor to
   a textarea that keeps the typed text across re-renders, and verify the
   PATCH body round-trips with newlines intact.

2. Add a reconciliation test for reordering keyed rows. The keyed algorithm
   moves existing nodes instead of re-mounting them, but no test exercises a
   reorder. Add a test that reorders a keyed list and asserts the nodes are
   moved (not re-created) and that the DOM order matches the new order.

## Done when

- A note body with newlines can be entered, saved, and re-rendered in a
  textarea without losing the typed text.
- A keyed-list reorder test passes and proves nodes are moved, not re-created.
- `go vet ./...`, `staticcheck ./...`, `go test ./...`, and `make test-web`
  all pass.
