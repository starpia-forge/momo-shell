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
	sshOpener := sshconn.New()
	localOpener := pty.NewOpener()

	hostSvc := host.New(hostRepo, secretStore, knownHostsRepo, sshOpener)
	sessionSvc := session.New(session.Deps{
		LocalOpener: localOpener,
		SSHOpener:   sshOpener,
		HostRepo:    hostRepo,
		Secrets:     secretStore,
		KnownHosts:  knownHostsRepo,
		Publisher:   publisher,
	})

	keyFileBrowser := wailsfacade.NewKeyFileBrowser()
	sessionService := wailsfacade.NewSessionService(sessionSvc)
	hostService := wailsfacade.NewHostService(hostSvc, keyFileBrowser)

	err = wailsapp.Run(&options.App{
		Title:  "momo-terminal",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup: func(ctx context.Context) {
			publisher.SetContext(ctx)
			keyFileBrowser.SetContext(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			sessionSvc.CloseAll()
			db.Close()
		},
		Bind: []interface{}{
			sessionService,
			hostService,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
