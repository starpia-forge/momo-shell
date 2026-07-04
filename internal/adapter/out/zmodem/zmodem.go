package zmodem

import "momo-shell/internal/core/port/out"

// Engine implements out.StreamTransfer. It carries no state between calls --
// each Send/Receive drives one complete exchange over the given conduit.
type Engine struct{}

func New() *Engine {
	return &Engine{}
}

var _ out.StreamTransfer = (*Engine)(nil)
