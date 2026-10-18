package main

import "sync"

// reliableRxWindow tracks the newest received reliable sequence and the previous
// 32 packets. It both builds the wire ACK bitmap and suppresses duplicate application
// dispatches, which otherwise can repeat purchases, damage, or inventory actions.
type reliableRxWindow struct {
	mu          sync.Mutex
	initialized bool
	latest      uint16
	bits        uint32
}

// record records seq and returns (ack, ackBits, duplicateOrTooOld). Bit 0 represents
// ack-1, bit 31 represents ack-32; uint16 subtraction intentionally wraps.
func (w *reliableRxWindow) record(seq uint16) (uint16, uint32, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.initialized {
		w.initialized = true
		w.latest = seq
		w.bits = 0
		return w.latest, w.bits, false
	}
	forward := int16(seq - w.latest)
	if forward > 0 {
		delta := uint16(forward)
		if delta > 32 {
			w.bits = 0
		} else {
			w.bits <<= delta
			w.bits |= uint32(1) << (delta - 1) // previous latest is now ack-delta
		}
		w.latest = seq
		return w.latest, w.bits, false
	}
	delta := uint16(w.latest - seq)
	if delta == 0 || delta > 32 {
		return w.latest, w.bits, true
	}
	mask := uint32(1) << (delta - 1)
	duplicate := w.bits&mask != 0
	w.bits |= mask
	return w.latest, w.bits, duplicate
}
