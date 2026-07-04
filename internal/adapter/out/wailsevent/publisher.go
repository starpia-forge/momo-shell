package wailsevent

import (
	"context"
	"encoding/base64"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Publisher implements out.EventPublisher via Wails' runtime.EventsEmit.
// Events published before SetContext is called (i.e. before OnStartup) are
// dropped rather than queued -- nothing is listening on the frontend yet.
type Publisher struct {
	ctx atomic.Pointer[context.Context]
}

func New() *Publisher {
	return &Publisher{}
}

// SetContext must be called from the Wails OnStartup hook before any
// session can be created.
func (p *Publisher) SetContext(ctx context.Context) {
	p.ctx.Store(&ctx)
}

// Publish emits topic to the frontend. []byte payloads (terminal output) are
// base64-encoded for binary-safe transport; everything else is passed as-is
// for Wails' own JSON marshaling.
func (p *Publisher) Publish(topic string, payload any) {
	ctxPtr := p.ctx.Load()
	if ctxPtr == nil {
		return
	}

	if b, ok := payload.([]byte); ok {
		runtime.EventsEmit(*ctxPtr, topic, base64.StdEncoding.EncodeToString(b))
		return
	}
	runtime.EventsEmit(*ctxPtr, topic, payload)
}
