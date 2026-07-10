package main

import (
	"context"
	"embed"
	"log"
	"path/filepath"
	"time"

	wailsapp "github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"momo-shell/internal/adapter/in/mcpipc"
	"momo-shell/internal/adapter/in/sharehttp"
	wailsfacade "momo-shell/internal/adapter/in/wails"
	"momo-shell/internal/adapter/out/keychain"
	"momo-shell/internal/adapter/out/mdns"
	"momo-shell/internal/adapter/out/pty"
	"momo-shell/internal/adapter/out/secretscan"
	"momo-shell/internal/adapter/out/sftp"
	"momo-shell/internal/adapter/out/shareclient"
	shellintegrationscripts "momo-shell/internal/adapter/out/shellintegration"
	"momo-shell/internal/adapter/out/shparse"
	"momo-shell/internal/adapter/out/sqlite"
	"momo-shell/internal/adapter/out/sshconn"
	"momo-shell/internal/adapter/out/wailsevent"
	"momo-shell/internal/adapter/out/zmodem"
	"momo-shell/internal/core/service/aicontrol"
	"momo-shell/internal/core/service/aicontrol/resolve"
	"momo-shell/internal/core/service/audit"
	"momo-shell/internal/core/service/capture"
	"momo-shell/internal/core/service/custodian"
	"momo-shell/internal/core/service/history"
	"momo-shell/internal/core/service/host"
	"momo-shell/internal/core/service/localfs"
	"momo-shell/internal/core/service/mask"
	"momo-shell/internal/core/service/scrollback"
	"momo-shell/internal/core/service/session"
	"momo-shell/internal/core/service/settings"
	"momo-shell/internal/core/service/share"
	"momo-shell/internal/core/service/shellintegration"
	"momo-shell/internal/core/service/transfer"
)

//go:embed all:frontend/dist
var assets embed.FS

// shellStateAdapter satisfies aicontrol.ShellStateReader by flattening
// shellintegration.Service.Query's map[string]VarValue (Set/Value pairs) to
// a plain map[string]string of currently-set vars -- the composition-root
// glue so aicontrol.state.go (B4) never imports the shellintegration
// package directly (SessionCreator's import-avoidance convention).
type shellStateAdapter struct{ si *shellintegration.Service }

func (a shellStateAdapter) ReadVars(ctx context.Context, sessionID string, names []string) (map[string]string, error) {
	vars, err := a.si.Query(ctx, sessionID, names)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(vars))
	for k, v := range vars {
		if v.Set {
			out[k] = v.Value
		}
	}
	return out, nil
}

// version is injected at build time via -ldflags "-X main.version=...".
var version = "dev"

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
	peerRepo := sqlite.NewPeerRepo(db)
	settingsRepo := sqlite.NewSettingsRepo(db)
	mcpClientRepo := sqlite.NewMCPClientRepo(db)
	auditRepo := sqlite.NewAuditRepo(db)
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
	// Runtime OSC133 shell-integration hooks (doc 17 §7.1: zmodem detection
	// runs first in the chain, shell-integration second) -- same two-phase,
	// consumer-defined-interface pattern as transferSvc above.
	shellIntegrationSvc := shellintegration.New(shellintegration.Deps{
		Shell:   sessionSvc,
		Builder: shellintegrationscripts.New(),
	})
	sessionSvc.AddMiddleware(shellIntegrationSvc)

	// AI-control wiring (doc 20 D1-D4, A-track): scrollback capture feeds
	// ReadScrollback's egress (masked via custodian+secretscan), resolve
	// gates RunCommand, and aicontrol.Service is both the MCP tool/resource
	// backend (mcpipc.Dispatch) and the pairing/auth callback target
	// (mcpipc.Server). AddTap must happen before any session exists (same
	// window as AddMiddleware above).
	scrollbackSvc := scrollback.New()
	sessionSvc.AddTap(scrollbackSvc)

	secretScanner := secretscan.New()
	custodianSvc := custodian.New(scrollbackSvc, secretStore)
	maskSvc := mask.New(mask.Deps{
		Scanner: secretScanner,
		Secrets: custodianSvc,
		// Commands (layer-2 command-aware gating) stays nil -- wiring it
		// needs aicontrol to expose its own in-flight command state, which
		// would introduce a two-phase aicontrol<->mask cycle; deferred to
		// its own cycle rather than bundled into this already-large wiring
		// pass. mask treats a nil Commands as a safe no-op (layer 1/3 still
		// fully active).
	})

	resolverSvc := resolve.New(resolve.Deps{Parser: shparse.New()})
	auditSvc := audit.New(auditRepo, secretStore, secretScanner)
	// E3-b: captures each command's original output in isolation from
	// scrollbackSvc's shared ring (see capture package doc for why), sealing
	// it to the command's audit row once RunCommand/lifecycle.go/
	// command_control.go drive its Begin/End. AddTap must happen before any
	// session exists (same window as scrollbackSvc.AddTap above).
	captureSvc := capture.New(auditSvc)
	sessionSvc.AddTap(captureSvc)

	aiSvc := aicontrol.New(aicontrol.Deps{
		Hosts:      hostRepo,
		Sessions:   sessionSvc,
		Publisher:  publisher,
		Resolver:   resolverSvc,
		Scrollback: scrollbackSvc,
		Masker:     maskSvc,
		Clients:    mcpClientRepo,
		ShellState: shellStateAdapter{si: shellIntegrationSvc}, // B4: GetShellState's cwd/env probe source
		Audit:      auditSvc,                                  // doc 18 E3: AI-control decision audit trail
		Capture:    captureSvc,                                // E3-b: original-output capture tap
	})
	shellIntegrationSvc.AddObserver(aiSvc) // D1: drive CommandHandle from OSC133 events

	mcpServer := mcpipc.New(aiSvc, mcpipc.NewDispatch(aiSvc))
	// SweepExpired is pure/deterministic and owns no timer itself (control.go
	// doc comment) -- this is the periodic caller its doc says lands with
	// whichever phase starts aicontrol's lifecycle.
	sweepTicker := time.NewTicker(time.Minute)
	sweepDone := make(chan struct{})

	shareSvc := share.New(share.Deps{
		HostRepo:   hostRepo,
		Clients:    shareClientRepo,
		Settings:   shareSettingsRepo,
		Pub:        publisher,
		Peers:      peerRepo,
		Secrets:    secretStore,
		Announcer:  mdns.NewAnnouncer(),
		Browser:    mdns.NewBrowser(),
		PeerClient: shareclient.New(),
	})
	shareCert, err := sharehttp.LoadOrCreateCert(filepath.Dir(dbPath))
	if err != nil {
		log.Fatalf("load or create share certificate: %v", err)
	}
	// Two-phase wiring breaks the shareSvc <-> shareServer construction cycle,
	// the same pattern as sessionSvc.SetMiddleware above: the server needs a
	// callback into shareSvc, and shareSvc needs to Start/Stop the server.
	shareServer := sharehttp.New(shareSvc, shareCert)
	shareSvc.SetServer(shareServer)

	settingsSvc := settings.New(settingsRepo)
	localfsSvc := localfs.New()

	keyFileBrowser := wailsfacade.NewKeyFileBrowser()
	clipboardWriter := wailsfacade.NewClipboardWriter()
	transferDialogs := wailsfacade.NewTransferDialogs()
	sessionService := wailsfacade.NewSessionService(sessionSvc)
	hostService := wailsfacade.NewHostService(hostSvc, keyFileBrowser)
	historyService := wailsfacade.NewHistoryService(historySvc)
	clipboardService := wailsfacade.NewClipboardService(clipboardWriter)
	transferService := wailsfacade.NewTransferService(transferSvc, transferDialogs)
	shareService := wailsfacade.NewShareService(shareSvc)
	settingsService := wailsfacade.NewSettingsService(settingsSvc, version)
	localFSService := wailsfacade.NewLocalFSService(localfsSvc)
	fileDropRelay := wailsfacade.NewFileDropRelay(publisher)
	mcpApprovalService := wailsfacade.NewMCPApprovalService(aiSvc)
	auditService := wailsfacade.NewAuditService(auditSvc) // E5: frontend-only audit-panel read path, not MCP-exposed

	err = wailsapp.Run(&options.App{
		Title:     "momo-shell",
		Frameless: true,
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
			if err := shareSvc.Start(); err != nil {
				log.Printf("share: start discovery: %v", err)
			}
			if err := mcpServer.Start(); err != nil {
				log.Printf("mcp: start ipc server: %v", err)
			}
			go func() {
				for {
					select {
					case <-sweepTicker.C:
						aiSvc.SweepExpired(time.Now())
						if err := auditSvc.PurgeOutputsBefore(time.Now()); err != nil {
							log.Printf("audit: purge expired outputs: %v", err)
						}
					case <-sweepDone:
						return
					}
				}
			}()
		},
		OnShutdown: func(ctx context.Context) {
			// Stop the MCP surface first so no in-flight AI request observes
			// a session/DB torn down out from under it.
			sweepTicker.Stop()
			close(sweepDone)
			_ = mcpServer.Stop(ctx)
			sessionSvc.CloseAll()
			historySvc.Close()
			_ = shareSvc.DisableSharing()
			shareSvc.Close()
			db.Close()
		},
		Bind: []interface{}{
			sessionService,
			hostService,
			historyService,
			clipboardService,
			transferService,
			shareService,
			settingsService,
			localFSService,
			mcpApprovalService,
			auditService,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
