package wails

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"momo-shell/internal/core/port/out"
)

// FileDropPayload is published on os:filedrop.
type FileDropPayload struct {
	X     int      `json:"x"`
	Y     int      `json:"y"`
	Paths []string `json:"paths"`
}

// FileDropRelay bridges Wails' native OS file-drop callback (which resolves
// absolute paths, unlike browser File objects) onto the same EventPublisher
// used for everything else -- the frontend subscribes to os:filedrop like
// any other topic instead of learning a second event mechanism.
type FileDropRelay struct {
	pub out.EventPublisher
}

func NewFileDropRelay(pub out.EventPublisher) *FileDropRelay {
	return &FileDropRelay{pub: pub}
}

// Register must be called from OnStartup, after the app's DragAndDrop
// options have EnableFileDrop set.
func (r *FileDropRelay) Register(ctx context.Context) {
	runtime.OnFileDrop(ctx, func(x, y int, paths []string) {
		r.pub.Publish(out.TopicOSFileDrop(), FileDropPayload{X: x, Y: y, Paths: paths})
	})
}
