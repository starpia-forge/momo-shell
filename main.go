package main

import (
	"context"
	"embed"
	"log"

	wailsapp "github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	wailsfacade "momo-terminal/internal/adapter/in/wails"
	"momo-terminal/internal/adapter/out/keychain"
	"momo-terminal/internal/adapter/out/pty"
	"momo-terminal/internal/adapter/out/sqlite"
	"momo-terminal/internal/adapter/out/sshconn"
	"momo-terminal/internal/adapter/out/wailsevent"
	"momo-terminal/internal/core/service/history"
	"momo-terminal/internal/core/service/host"
	"momo-terminal/internal/core/service/session"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Composition root: wire ports to adapters. Nothing above this function
	// (internal/core/...) knows about Wails or any concrete adapter.
	dbPath, err := sqlite.DefaultPath()
	if err != nil {
		log.Fatalf("resolve database path: %v", err)
	}
	db, err := sqlite.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	secretStore, err := keychain.New()
	if err != nil {
		log.Fatalf("open secret store: %v", err)
	}

	publisher := wailsevent.New()
	hostRepo := sqlite.NewHostRepo(db)
	knownHostsRepo := sqlite.NewKnownHostsRepo(db)
	historyRepo := sqlite.NewHistoryRepo(db)
	sshOpener := sshconn.New()
	localOpener := pty.NewOpener()

	hostSvc := host.New(hostRepo, secretStore, knownHostsRepo, sshOpener)
	historySvc := history.New(historyRepo, publisher)
	sessionSvc := session.New(session.Deps{
		LocalOpener: localOpener,
		SSHOpener:   sshOpener,
		HostRepo:    hostRepo,
		Secrets:     secretStore,
		KnownHosts:  knownHostsRepo,
		Publisher:   publisher,
		Tap:         historySvc,
	})

	keyFileBrowser := wailsfacade.NewKeyFileBrowser()
	clipboardWriter := wailsfacade.NewClipboardWriter()
	sessionService := wailsfacade.NewSessionService(sessionSvc)
	hostService := wailsfacade.NewHostService(hostSvc, keyFileBrowser)
	historyService := wailsfacade.NewHistoryService(historySvc)
	clipboardService := wailsfacade.NewClipboardService(clipboardWriter)

	err = wailsapp.Run(&options.App{
		Title:     "momo-terminal",
		Width:     1024,
		Height:    768,
		MinWidth:  640,
		MinHeight: 480,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup: func(ctx context.Context) {
			publisher.SetContext(ctx)
			keyFileBrowser.SetContext(ctx)
			clipboardWriter.SetContext(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			sessionSvc.CloseAll()
			historySvc.Close()
			db.Close()
		},
		Bind: []interface{}{
			sessionService,
			hostService,
			historyService,
			clipboardService,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
