// Package zmodem implements a subset of the ZMODEM file transfer protocol
// (Chuck Forsberg's zmodem.doc) sufficient to interoperate with lrzsz's
// rz/sz: ZRQINIT, ZRINIT, ZFILE, ZDATA, ZEOF, ZFIN, ZACK, ZNAK, ZCAN, plus
// CRC32 binary framing. It never touches SSH or any other transport --
// Send/Receive run over a plain io.ReadWriter (see out.StreamTransfer).
package zmodem

// Byte-level framing constants.
const (
	zpad  = '*'  // 0x2a -- pads before ZDLE; we always emit two
	zdle  = 0x18 // CAN, reused as the ZMODEM escape character
	zdlee = zdle ^ 0x40

	// Header format selectors, sent as ZDLE <selector>.
	zbin   = 'A' // binary header, 16-bit CRC (unused -- we always prefer 32-bit)
	zhex   = 'B' // hex header, 16-bit CRC (used for the initial handshake)
	zbin32 = 'C' // binary header, 32-bit CRC (used for everything else)

	xon = 0x11
)

// Frame types (the header's first byte).
const (
	zrqinit    = 0
	zrinit     = 1
	zsinit     = 2
	zack       = 3
	zfile      = 4
	zskip      = 5
	znak       = 6
	zabort     = 7
	zfin       = 8
	zrpos      = 9
	zdata      = 10
	zeof       = 11
	zferr      = 12
	zcrc       = 13
	zchallenge = 14
	zcompl     = 15
	zcan       = 16
	zfreecnt   = 17
	zcommand   = 18
	zstderr    = 19
)

// ZRINIT capability flags (header data byte 3, "F0").
const (
	canFDX  = 0x01 // can send and receive concurrently
	canOvIO = 0x02
	canBrk  = 0x04
	canCry  = 0x08
	canLZW  = 0x10
	canFC32 = 0x20 // supports 32-bit CRC frames
	escCtl  = 0x40
	esc8    = 0x80
)

// Data-subpacket terminators (the byte following escaped payload data,
// before its CRC).
const (
	zcrce = 'h' // frame end, no ZACK expected, next header follows immediately
	zcrcg = 'i' // frame continues, no ZACK expected (streaming)
	zcrcq = 'j' // frame continues, ZACK expected
	zcrcw = 'k' // frame end, ZACK expected
)

// cancelSequence aborts a ZMODEM exchange: enough CANs to break out of any
// subpacket or header the far end might be mid-parsing, per lrzsz convention.
var cancelSequence = []byte{zdle, zdle, zdle, zdle, zdle, 8, 8, 8, 8, 8}

const subpacketMaxLen = 1024
