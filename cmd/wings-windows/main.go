package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"syscall"
	"time"
	"wings-windows/internal/api"
	"wings-windows/internal/config"
	"wings-windows/internal/identity"
	"wings-windows/internal/node"
	"wings-windows/internal/provision"
)

func run(ctx context.Context, path, markInstalled string) error {
	return runWithReady(ctx, path, markInstalled, func() {})
}

type runtimeFailure struct {
	stage string
	err   error
}

func (e *runtimeFailure) Error() string { return e.err.Error() }
func (e *runtimeFailure) Unwrap() error { return e.err }

func runWithReady(ctx context.Context, path, markInstalled string, ready func()) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("native Windows runtime requires Windows")
	}
	c, err := config.Load(path)
	if err != nil {
		return &runtimeFailure{"configuration", err}
	}
	m := node.NewManager(c)
	defer m.Close()
	if err = m.Sync(ctx); err != nil {
		return &runtimeFailure{"panel_sync", fmt.Errorf("initial panel sync: %w", err)}
	}
	if markInstalled != "" {
		s := m.Get(markInstalled)
		if s == nil {
			return fmt.Errorf("server must belong to this node and have a local profile")
		}
		if err := provision.Check(ctx, s.Profile); err != nil {
			return fmt.Errorf("profile preflight before installation acknowledgement: %w", err)
		}
		return m.Panel.Installed(ctx, s.ID())
	}
	handler := api.New(m)
	defer handler.Close()
	server := &http.Server{Addr: c.Listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 * 1024}
	if c.TLS.Cert != "" {
		certificate, err := tls.LoadX509KeyPair(c.TLS.Cert, c.TLS.Key)
		if err != nil {
			return &runtimeFailure{"tls", err}
		}
		server.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}}
	}
	listener, err := net.Listen("tcp", c.Listen)
	if err != nil {
		return &runtimeFailure{"listener", err}
	}
	defer listener.Close()
	if err := ctx.Err(); err != nil {
		return err
	}
	result := make(chan error, 1)
	go func() {
		log.Printf("Windows Native %s listening on %s (%d configured servers)", config.Version, c.Listen, len(m.All()))
		if c.TLS.Cert != "" {
			result <- server.ServeTLS(listener, "", "")
		} else {
			result <- server.Serve(listener)
		}
	}()
	ready()
	m.RestoreRunning(ctx)
	select {
	case err = <-result:
		if err == http.ErrServerClosed {
			return nil
		}
		return &runtimeFailure{"http", err}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handler.Close()
		err := server.Shutdown(shutdown)
		if err != nil {
			server.Close()
		}
		if err != nil {
			return &runtimeFailure{"shutdown", err}
		}
		return nil
	}
}
func main() {
	path := flag.String("config", "config.json", "Path to node configuration")
	check := flag.Bool("check", false, "Validate configuration without contacting the panel")
	checkProfiles := flag.Bool("check-profiles", false, "Check local profile executables, Windows requirements and dependency files without contacting the panel")
	version := flag.Bool("version", false, "Print version")
	installed := flag.String("mark-installed", "", "Explicitly acknowledge manually provisioned server UUID to the panel")
	credentialStatus := flag.String("credential-status", "", "Report credential metadata for one full server UUID without account logon")
	removeCredential := flag.String("remove-credential", "", "Remove the current account's exact per-server credential; does not stop games or delete users")
	rconStatus := flag.String("rcon-credential-status", "", "Report metadata for the current account's exact server RCON credential")
	rconRemove := flag.String("remove-rcon-credential", "", "Remove only the current account's exact server RCON credential")
	checkIdentity := flag.String("check-identity", "", "Diagnose one configured identity with a batch logon and access checks, without starting a game")
	checkNetwork := flag.String("check-network-policy", "", "Temporarily install/inspect/remove the configured account block after identity preflight; no traffic probe")
	flag.Parse()
	if flag.NArg() != 0 {
		log.Fatal("unexpected positional arguments")
	}
	flag.Visit(func(f *flag.Flag) {
		if (f.Name == "credential-status" || f.Name == "remove-credential" || f.Name == "rcon-credential-status" || f.Name == "remove-rcon-credential" || f.Name == "check-identity" || f.Name == "check-network-policy") && f.Value.String() == "" {
			log.Fatal("credential operation requires a full server UUID")
		}
	})
	operations := 0
	for _, selected := range []bool{*version, *check, *checkProfiles, *installed != "", *credentialStatus != "", *removeCredential != "", *rconStatus != "", *rconRemove != "", *checkIdentity != "", *checkNetwork != ""} {
		if selected {
			operations++
		}
	}
	if operations > 1 {
		log.Fatal("choose only one CLI operation")
	}
	if *rconStatus != "" {
		metadata, err := identity.RCONCredentialStatus(*rconStatus)
		if err != nil {
			log.Fatal(err)
		}
		if err = json.NewEncoder(os.Stdout).Encode(metadata); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *rconRemove != "" {
		removed, err := identity.RemoveRCONCredential(*rconRemove)
		if err != nil {
			log.Fatal(err)
		}
		target, _ := identity.RCONCredentialTarget(*rconRemove)
		if err = json.NewEncoder(os.Stdout).Encode(map[string]any{"target": target, "removed": removed}); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *credentialStatus != "" {
		metadata, err := identity.CredentialStatus(*credentialStatus)
		if err != nil {
			log.Fatal(err)
		}
		if err := json.NewEncoder(os.Stdout).Encode(metadata); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *removeCredential != "" {
		removed, err := identity.RemoveCredential(*removeCredential)
		if err != nil {
			log.Fatal(err)
		}
		target, _ := identity.CredentialTarget(*removeCredential)
		if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"target": target, "removed": removed}); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *version {
		fmt.Println(config.Version)
		return
	}
	if *checkIdentity != "" {
		c, err := config.Load(*path)
		if err != nil {
			log.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		report := node.CheckIdentity(ctx, c, *checkIdentity)
		if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
			log.Fatal(err)
		}
		if !report.Passed {
			os.Exit(1)
		}
		return
	}
	if *checkNetwork != "" {
		c, err := config.Load(*path)
		if err != nil {
			log.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		report := node.CheckNetworkPolicy(ctx, c, *checkNetwork)
		if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
			log.Fatal(err)
		}
		if !report.Passed {
			os.Exit(1)
		}
		return
	}
	if *check {
		if _, err := config.Load(*path); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Configuration valid")
		return
	}
	if *checkProfiles {
		c, err := config.Load(*path)
		if err != nil {
			log.Fatal(err)
		}
		ids := make([]string, 0, len(c.Profiles))
		for id := range c.Profiles {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		failed := false
		for _, id := range ids {
			if err := provision.Check(context.Background(), c.Profiles[id]); err != nil {
				fmt.Fprintf(os.Stderr, "%s: FAILED: %v\n", id, err)
				failed = true
			} else {
				fmt.Printf("%s: profile preflight passed\n", id)
			}
		}
		if failed {
			os.Exit(1)
		}
		return
	}
	if serviceMain(*path, *installed) {
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, *path, *installed); err != nil {
		log.Fatal(err)
	}
}
