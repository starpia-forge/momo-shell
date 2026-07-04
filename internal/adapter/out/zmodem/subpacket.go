package zmodem

import (
	"fmt"
	"io"
)

// writeSubpacket sends one ZDATA data subpacket: escaped payload bytes,
// then ZDLE + term, then a 32-bit CRC covering payload+term.
func writeSubpacket(w io.Writer, payload []byte, term byte) error {
	buf := make([]byte, 0, len(payload)*2+8)
	for _, b := range payload {
		buf = appendEscaped(buf, b)
	}
	buf = append(buf, zdle, term)

	crc := crc32Of(append(append([]byte{}, payload...), term))
	crcBytes := []byte{byte(crc), byte(crc >> 8), byte(crc >> 16), byte(crc >> 24)}
	for _, b := range crcBytes {
		buf = appendEscaped(buf, b)
	}

	_, err := w.Write(buf)
	return err
}

// readSubpacket reads escaped data bytes until a terminator (ZDLE + one of
// zcrce/zcrcg/zcrcq/zcrcw), then verifies the 32-bit CRC that follows.
func (f *frameReader) readSubpacket() (payload []byte, term byte, err error) {
	for {
		b, err := f.readByte()
		if err != nil {
			return nil, 0, err
		}
		if b != zdle {
			payload = append(payload, b)
			continue
		}

		e, err := f.readByte()
		if err != nil {
			return nil, 0, err
		}
		switch e {
		case zcrce, zcrcg, zcrcq, zcrcw:
			term = e
		case zdle:
			return nil, 0, errCanceled
		default:
			payload = append(payload, e^0x40)
			continue
		}
		break
	}

	crcBytes := make([]byte, 4)
	for i := range crcBytes {
		b, err := f.readEscapedByte()
		if err != nil {
			return nil, 0, err
		}
		crcBytes[i] = b
	}
	want := uint32(crcBytes[0]) | uint32(crcBytes[1])<<8 | uint32(crcBytes[2])<<16 | uint32(crcBytes[3])<<24
	got := crc32Of(append(append([]byte{}, payload...), term))
	if got != want {
		return nil, 0, fmt.Errorf("zmodem: subpacket CRC mismatch")
	}
	return payload, term, nil
}
