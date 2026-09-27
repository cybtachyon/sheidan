package bodycap

import (
	"io"
	"testing"
)

// scriptedSupplier plays a stream that hands out its bytes in
// fixed-size drips, mimicking a socket that dribbles.
type scriptedSupplier struct {
	payload []byte
	chunk   int
	pos     int
}

func (sp *scriptedSupplier) Read(p []byte) (int, error) {
	if sp.pos >= len(sp.payload) {
		return 0, io.EOF
	}
	end := sp.pos + sp.chunk
	if end > len(sp.payload) {
		end = len(sp.payload)
	}
	n := end - sp.pos
	copy(p, sp.payload[sp.pos:end])
	sp.pos = end
	return n, nil
}

// TestMeteredDeliveryUnderCap verifies a stream that ends exactly at
// the cap is judged compliant: delivery stops, no over flag.
func TestMeteredDeliveryUnderCap(t *testing.T) {
	supplier := &scriptedSupplier{payload: []byte("abcd"), chunk: 1}
	reader := NewReader(supplier, 4)
	got := make([]byte, 8)
	total := 0
	for {
		n, err := reader.Read(got)
		total += n
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("mid-delivery error = %v", err)
		}
	}
	if total != 4 {
		t.Fatalf("delivered = %d; want 4", total)
	}
	if reader.Over() {
		t.Error("stream ending at the cap flagged as over")
	}
}

// TestMeteredDeliveryPastCap verifies a stream that owes more than
// the cap delivers only the cap, then EOF, with the over flag set.
func TestMeteredDeliveryPastCap(t *testing.T) {
	supplier := &scriptedSupplier{payload: []byte("abcdefghij"), chunk: 3}
	reader := NewReader(supplier, 4)
	buffer := make([]byte, 8)
	delivered := 0
	for {
		n, err := reader.Read(buffer)
		delivered += n
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("mid-delivery error = %v", err)
		}
	}
	if delivered != 4 {
		t.Fatalf("delivered = %d; want 4", delivered)
	}
	if !reader.Over() {
		t.Error("oversized stream not flagged as over")
	}
}

// TestMeteredDeliveryShortStream verifies a stream shorter than the
// cap passes wholly, terminating with the supplier's own EOF and no
// over flag.
func TestMeteredDeliveryShortStream(t *testing.T) {
	supplier := &scriptedSupplier{payload: []byte("xy"), chunk: 1}
	reader := NewReader(supplier, 10)
	buffer := make([]byte, 8)
	total := 0
	for {
		n, err := reader.Read(buffer)
		total += n
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("mid-delivery error = %v", err)
		}
	}
	if total != 2 {
		t.Fatalf("delivered = %d; want 2", total)
	}
	if reader.Over() {
		t.Error("undersized stream flagged as over")
	}
}
