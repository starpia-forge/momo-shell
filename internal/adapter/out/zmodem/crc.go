package zmodem

import "hash/crc32"

// crc16Table implements the CRC-CCITT (poly 0x1021) variant ZMODEM inherited
// from XMODEM/CRC for its 16-bit hex-header mode.
var crc16Table [256]uint16

func init() {
	const poly = 0x1021
	for i := 0; i < 256; i++ {
		crc := uint16(i) << 8
		for j := 0; j < 8; j++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ poly
			} else {
				crc <<= 1
			}
		}
		crc16Table[i] = crc
	}
}

func updateCRC16(crc uint16, b byte) uint16 {
	return (crc << 8) ^ crc16Table[byte(crc>>8)^b]
}

// crc16 is the standard CRC-16/XMODEM ZMODEM uses for its hex headers:
// poly 0x1021, init 0, no reflection, no final XOR, no augmentation.
// (Verified against real lrzsz output -- an earlier version of this
// function added a two-zero-byte "flush" step that doesn't match reality.)
func crc16(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc = updateCRC16(crc, b)
	}
	return crc
}

// crc32Of is ZMODEM's 32-bit CRC mode: the same CRC-32 algorithm used by
// zip/gzip/ethernet (unlike the 16-bit mode, no zero-byte flush).
func crc32Of(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}
