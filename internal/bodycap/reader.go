package bodycap

import "io"

// Reader meters a body stream against a cap. Delivery stops at the
// cap, feeding downstream decoders EOF instead of a runaway
// allocation. The next read after the cap probes the supplier for
// one more byte, distinguishing a stream that merely ended at the
// cap from one that owed more: the latter flips the Over flag, which
// the slot converts into a 413 verdict.
type Reader struct {
	in        io.Reader
	left      int64
	limit     int64
	over      bool
	exhausted bool
	probe     []byte
}

// NewReader arms a metering wrapper over in, allowing at most limit
// bytes.
func NewReader(in io.Reader, limit int64) *Reader {
	return &Reader{in: in, left: limit, limit: limit, probe: make([]byte, 1)}
}

// Over reports whether the supplier offered more bytes than the cap
// admitted.
func (rd *Reader) Over() bool { return rd.over }

// Read implements io.Reader with the metering discipline.
func (rd *Reader) Read(p []byte) (int, error) {
	if rd.exhausted {
		return 0, io.EOF
	}
	if rd.left <= 0 {
		// The cap spent its credit. Probe whether the supplier
		// still owes bytes before settling the verdict: a
		// supplier that ends exactly at the cap is compliant,
		// and its decoder merits the gentler truncation error.
		n, err := rd.in.Read(rd.probe)
		if n > 0 || (err != nil && err != io.EOF) {
			rd.over = true
		}
		rd.exhausted = true
		return 0, io.EOF
	}
	if int64(len(p)) > rd.left {
		p = p[:rd.left]
	}
	n, err := rd.in.Read(p)
	if n > len(p) {
		// A rogue supplier may claim more than the room
		// it was handed; clip to the room, not the claim.
		n = len(p)
	}
	rd.left -= int64(n)
	if rd.left == 0 && err == nil {
		// Credit spent, but the supplier may owe more; the
		// probe on the next call settles it.
		return n, nil
	}
	return n, err
}

// Close shuts the supplier underneath, fulfilling the io.ReadCloser
// contract the server's connection recycling expects of a swapped
// body.
func (rd *Reader) Close() error {
	type closable interface{ Close() error }
	if cs, ok := rd.in.(closable); ok {
		return cs.Close()
	}
	return nil
}
