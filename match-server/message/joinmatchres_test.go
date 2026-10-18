package message

import (
	"encoding/binary"
	"testing"
)

// Regression test for LGIGCGIDOKP field alignment. The client crashed in
// EKHEMGJLHDC.ReadString when the 64-bit EOKOPECMPDF field was omitted.
func TestJoinMatchResFieldAlignment(t *testing.T) {
	b := JoinMatchRes(0, 0x11223344, 0x55667788)
	if len(b) != 129 {
		t.Fatalf("JoinMatchRes length = %d, want 129", len(b))
	}
	// Prefix through FMMDBCOBGDM: result + 3 ulong + int32 + byte + uint32.
	if got := binary.LittleEndian.Uint32(b[0:4]); got != 0 {
		t.Fatalf("result = %#x, want 0", got)
	}
	// EOKOPECMPDF must occupy eight bytes after the ushort at [37:39].
	if got := binary.LittleEndian.Uint64(b[39:47]); got != 0 {
		t.Fatalf("EOKOPECMPDF = %#x, want 0", got)
	}
	if got := binary.LittleEndian.Uint32(b[47:51]); got != 0 {
		t.Fatalf("CKAAOCLHHMM = %#x, want 0", got)
	}
	if got := binary.LittleEndian.Uint32(b[51:55]); got != 0 {
		t.Fatalf("DPOPAMPEKGA = %#x, want 0", got)
	}
	// These room-setting values are deliberately distinctive so offsets regress visibly.
	if got := binary.LittleEndian.Uint32(b[95:99]); got != 0x11223344 {
		t.Fatalf("roomSetting = %#x, want 0x11223344", got)
	}
	if got := binary.LittleEndian.Uint32(b[99:103]); got != 0x55667788 {
		t.Fatalf("roomSetting2 = %#x, want 0x55667788", got)
	}
	// The nested message ends with its bool, followed by two LGIGCGIDOKP fields.
	if b[126] != 0 || b[127] != 0 || b[128] != 0 {
		t.Fatalf("trailing fields = %v, want [0 0 0]", b[126:129])
	}
}
