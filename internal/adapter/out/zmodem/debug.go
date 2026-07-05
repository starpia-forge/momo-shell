package zmodem

import (
	"fmt"
	"os"
)

// debugEnabled gates the zmodem package's diagnostic logging behind an
// environment variable -- off by default so it costs nothing in normal
// operation, but available for capturing exactly what bytes/frames a real
// session exchanged the next time an intermittent interop issue (like
// FT-11's occasional first-detection miss) needs to be pinned down.
var debugEnabled = os.Getenv("MOMO_ZMODEM_DEBUG") == "1"

func debugf(format string, args ...any) {
	if !debugEnabled {
		return
	}
	fmt.Fprintf(os.Stderr, "[zmodem] "+format+"\n", args...)
}

// debugHexDump renders up to max bytes of b as hex, truncating with an
// ellipsis marker rather than dumping an entire (potentially large) buffer.
func debugHexDump(b []byte, max int) string {
	if len(b) > max {
		return fmt.Sprintf("% x...(+%d more bytes)", b[:max], len(b)-max)
	}
	return fmt.Sprintf("% x", b)
}
