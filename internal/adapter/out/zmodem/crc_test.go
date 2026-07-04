package zmodem

import "testing"

func TestCRC16_Deterministic(t *testing.T) {
	data := []byte("The quick brown fox jumps over the lazy dog")
	if crc16(data) != crc16(data) {
		t.Fatalf("crc16 should be deterministic for the same input")
	}
}

func TestCRC16_EmptyInput(t *testing.T) {
	// Deterministic and shouldn't panic on empty data (e.g. a zero-length subpacket).
	if crc16(nil) != crc16(nil) {
		t.Fatalf("crc16(nil) should be deterministic")
	}
}

func TestCRC32_KnownCheckValue(t *testing.T) {
	// The standard CRC-32/ISO-HDLC check value for the ASCII string "123456789".
	got := crc32Of([]byte("123456789"))
	const want = 0xCBF43926
	if got != want {
		t.Fatalf("crc32Of(\"123456789\") = %08x, want %08x", got, want)
	}
}
