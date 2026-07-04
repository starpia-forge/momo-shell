package wails

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ClipboardWriter owns the Wails context needed for clipboard access. It is
// a separate, unbound type (like KeyFileBrowser) specifically so its
// SetContext method never ends up in ClipboardService's bound method set --
// Wails binds every exported method of a bound struct.
type ClipboardWriter struct {
	ctx atomic.Pointer[context.Context]
}

func NewClipboardWriter() *ClipboardWriter {
	return &ClipboardWriter{}
}

// SetContext must be called from OnStartup before SetText is used.
func (w *ClipboardWriter) SetContext(ctx context.Context) {
	w.ctx.Store(&ctx)
}

func (w *ClipboardWriter) SetText(text string) error {
	ctxPtr := w.ctx.Load()
	if ctxPtr == nil {
		return errors.New("clipboard: context not set")
	}
	return runtime.ClipboardSetText(*ctxPtr, text)
}

// ClipboardService is the Wails-bound facade over ClipboardWriter. Go-side
// runtime.ClipboardSetText is used instead of the frontend's
// navigator.clipboard because WebView2's clipboard-write permission
// prompting has proven inconsistent in this app's embedding, while the OS
// clipboard API is deterministic.
type ClipboardService struct {
	writer *ClipboardWriter
}

func NewClipboardService(writer *ClipboardWriter) *ClipboardService {
	return &ClipboardService{writer: writer}
}

func (s *ClipboardService) SetClipboardText(text string) error {
	return s.writer.SetText(text)
}
