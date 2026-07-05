package zmodem

import "momo-shell/internal/core/port/out"

// Engine implements out.StreamTransfer. It carries no state between calls --
// each Send/Receive drives one complete exchange over the given conduit.
type Engine struct{}

func New() *Engine {
	return &Engine{}
}

// CancelBytes returns the ZMODEM cancel sequence (see cancelSequence in
// consts.go).
func (e *Engine) CancelBytes() []byte {
	return append([]byte{}, cancelSequence...)
}

var _ out.StreamTransfer = (*Engine)(nil)
