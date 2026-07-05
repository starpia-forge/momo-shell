package main

import (
	"context"
	"embed"
	"log"
	"path/filepath"

	wailsapp "github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"momo-shell/internal/adapter/in/sharehttp"
	wailsfacade "momo-shell/internal/adapter/in/wails"
	"momo-shell/internal/adapter/out/keychain"
	"momo-shell/internal/adapter/out/pty"
	"momo-shell/internal/adapter/out/sftp"
	"momo-shell/internal/adapter/out/sqlite"
	"momo-shell/internal/adapter/out/sshconn"
	"momo-shell/internal/adapter/out/wailsevent"
	"momo-shell/internal/adapter/out/zmodem"
	"momo-shell/internal/core/service/history"
	"momo-shell/internal/core/service/host"
	"momo-shell/internal/core/service/session"
	"momo-shell/internal/core/service/share"
	"momo-shell/internal/core/service/transfer"
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
	shareClientRepo := sqlite.NewShareClientRepo(db)
	shareSettingsRepo := sqlite.NewShareSettingsRepo(db)
	sshOpener := sshconn.New(sshconn.WithFileSystemFactory(sftp.NewFromClient))
	localOpener := pty.NewOpener()

	hostSvc := host.New(hostRepo, secretStore, knownHostsRepo, sshOpener)
	historySvc := history.New(historyRepo, publisher)
	// Two-phase wiring breaks the sessionSvc <-> transferSvc cycle: transferSvc
	// needs sessionSvc as its ShellAccess, and sessionSvc needs transferSvc as
	// its output middleware (for ZMODEM detection). Safe only because this
	// all happens before wailsapp.Run -- no session exists yet.
	sessionSvc := session.New(session.Deps{
		LocalOpener: localOpener,
		SSHOpener:   sshOpener,
		HostRepo:    hostRepo,
		Secrets:     secretStore,
		KnownHosts:  knownHostsRepo,
		Publisher:   publisher,
		Tap:         historySvc,
	})
	transferSvc := transfer.New(transfer.Deps{Shell: sessionSvc, Pub: publisher, Zmodem: zmodem.New()})
	sessionSvc.SetMiddleware(transferSvc)

	shareSvc := share.New(share.Deps{HostRepo: hostRepo, Clients: shareClientRepo, Settings: shareSettingsRepo, Pub: publisher})
	shareCert, err := sharehttp.LoadOrCreateCert(filepath.Dir(dbPath))
	if err != nil {
		log.Fatalf("load or create share certificate: %v", err)
	}
	// Two-phase wiring breaks the shareSvc <-> shareServer construction cycle,
	// the same pattern as sessionSvc.SetMiddleware above: the server needs a
	// callback into shareSvc, and shareSvc needs to Start/Stop the server.
	shareServer := sharehttp.New(shareSvc, shareCert)
	shareSvc.SetServer(shareServer)

	keyFileBrowser := wailsfacade.NewKeyFileBrowser()
	clipboardWriter := wailsfacade.NewClipboardWriter()
	transferDialogs := wailsfacade.NewTransferDialogs()
	sessionService := wailsfacade.NewSessionService(sessionSvc)
	hostService := wailsfacade.NewHostService(hostSvc, keyFileBrowser)
	historyService := wailsfacade.NewHistoryService(historySvc)
	clipboardService := wailsfacade.NewClipboardService(clipboardWriter)
	transferService := wailsfacade.NewTransferService(transferSvc, transferDialogs)
	shareService := wailsfacade.NewShareService(shareSvc)
	fileDropRelay := wailsfacade.NewFileDropRelay(publisher)

	err = wailsapp.Run(&options.App{
		Title:     "momo-shell",
		Width:     1024,
		Height:    768,
		MinWidth:  640,
		MinHeight: 480,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
		},
		OnStartup: func(ctx context.Context) {
			publisher.SetContext(ctx)
			keyFileBrowser.SetContext(ctx)
			clipboardWriter.SetContext(ctx)
			transferDialogs.SetContext(ctx)
			fileDropRelay.Register(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			sessionSvc.CloseAll()
			historySvc.Close()
			_ = shareSvc.DisableSharing()
			db.Close()
		},
		Bind: []interface{}{
			sessionService,
			hostService,
			historyService,
			clipboardService,
			transferService,
			shareService,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
