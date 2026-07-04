package main

import (
	"context"
	"embed"

	wailsapp "github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	sessionfacade "momo-terminal/internal/adapter/in/wails"
	"momo-terminal/internal/adapter/out/pty"
	"momo-terminal/internal/adapter/out/wailsevent"
	"momo-terminal/internal/core/service/session"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Composition root: wire ports to adapters. Nothing above this function
	// (internal/core/...) knows about Wails or any concrete adapter.
	publisher := wailsevent.New()
	opener := pty.NewOpener()
	sessionSvc := session.New(session.Deps{
		LocalOpener: opener,
		Publisher:   publisher,
	})
	sessionService := sessionfacade.NewSessionService(sessionSvc)

	err := wailsapp.Run(&options.App{
		Title:  "momo-terminal",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup: func(ctx context.Context) {
			publisher.SetContext(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			sessionSvc.CloseAll()
		},
		Bind: []interface{}{
			sessionService,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
