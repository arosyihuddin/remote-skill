package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pstar7/remote-skill/internal/audit"
	"github.com/pstar7/remote-skill/internal/config"
	"github.com/pstar7/remote-skill/internal/handlers"
	"github.com/pstar7/remote-skill/internal/node"
	"github.com/pstar7/remote-skill/internal/policy"
	"github.com/pstar7/remote-skill/internal/proto"
)

func runDaemon(args []string) {
	cfgPath := flag.String("config", defaultConfigPath(), "path to config file")
	flag.Parse()

	cfg, err := config.LoadNodeConfig(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	pol, err := policy.New(cfg.AllowCmd, cfg.DenyCmd, cfg.DenyPath, cfg.AllowGUI)
	if err != nil {
		log.Fatalf("policy: %v", err)
	}
	if len(cfg.AllowCmd) > 0 {
		log.Printf("policy: ALLOW_CMD active (%d patterns)", len(cfg.AllowCmd))
	}
	if len(cfg.DenyCmd) > 0 {
		log.Printf("policy: DENY_CMD active (%d patterns)", len(cfg.DenyCmd))
	}
	if len(cfg.DenyPath) > 0 {
		log.Printf("policy: DENY_PATH active (%d patterns)", len(cfg.DenyPath))
	}
	var auditLog *audit.Logger
	if cfg.AuditEnabled && cfg.AuditPath != "" {
		auditLog, err = audit.Open(cfg.AuditPath)
		if err != nil {
			log.Fatalf("audit: %v", err)
		}
		defer auditLog.Close()
		log.Printf("audit: writing to %s", cfg.AuditPath)
	}

	deps := &handlers.Deps{
		Policy:   pol,
		Audit:    auditLog,
		DeviceID: cfg.DeviceID,
	}

	n := node.New(cfg.ServerURL, cfg.DeviceID, cfg.Token)

	n.Register(proto.TypeExec, deps.WrapExec())
	n.Register(proto.TypeReadFile, deps.WrapReadFile())
	n.Register(proto.TypeWriteFile, deps.WrapWriteFile())
	n.Register(proto.TypeListDir, deps.WrapListDir())
	n.Register(proto.TypeScreenshot, deps.WrapScreenshot())
	n.Register(proto.TypeClick, deps.WrapClick())
	n.Register(proto.TypeType, deps.WrapType())
	n.Register(proto.TypeKey, deps.WrapKey())
	n.Register(proto.TypeMouse, deps.WrapMouse())
	n.Register(proto.TypeScroll, deps.WrapScroll())
	n.Register(proto.TypeClipboardRead, deps.WrapClipboardRead())
	n.Register(proto.TypeClipboardWrite, deps.WrapClipboardWrite())
	n.Register(proto.TypeDrag, deps.WrapDrag())
	n.Register(proto.TypeBoard, deps.WrapBoard())
	n.Register(proto.TypeWindows, deps.WrapWindows())
	n.Register(proto.TypeAccessibilityTree, deps.WrapAccessibilityTree())
	n.Register(proto.TypeMonitors, deps.WrapMonitors())
	n.Register(proto.TypeCursorPos, deps.WrapCursorPos())
	n.Register(proto.TypeAppList, deps.WrapAppList())
	n.Register(proto.TypeAppLaunch, deps.WrapAppLaunch())
	n.Register(proto.TypeCloseWindow, deps.WrapCloseWindow())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down")
		cancel()
	}()

	if err := n.RunForever(ctx, time.Duration(cfg.ReconnectSec)*time.Second); err != nil && err != context.Canceled {
		log.Fatalf("node stopped: %v", err)
	}
}
