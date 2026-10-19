package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
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
	if runtime.GOOS != "windows" {
		return fmt.Errorf("native Windows runtime requires Windows")
	}
	c, err := config.Load(path)
	if err != nil {
		return err
	}
	m := node.NewManager(c)
	if err = m.Sync(ctx); err != nil {
		return fmt.Errorf("initial panel sync: %w", err)
	}
	defer m.Close()
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
	result := make(chan error, 1)
	go func() {
		log.Printf("Windows Native %s listening on %s (%d configured servers)", config.Version, c.Listen, len(m.All()))
		if c.TLS.Cert != "" {
			result <- server.ListenAndServeTLS(c.TLS.Cert, c.TLS.Key)
		} else {
			result <- server.ListenAndServe()
		}
	}()
	select {
	case err = <-result:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handler.Close()
		err := server.Shutdown(shutdown)
		if err != nil {
			server.Close()
		}
		return err
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
	checkIdentity := flag.String("check-identity", "", "Diagnose one configured identity with a batch logon and access checks, without starting a game")
	flag.Parse()
	if flag.NArg() != 0 {
		log.Fatal("unexpected positional arguments")
	}
	flag.Visit(func(f *flag.Flag) {
		if (f.Name == "credential-status" || f.Name == "remove-credential" || f.Name == "check-identity") && f.Value.String() == "" {
			log.Fatal("credential operation requires a full server UUID")
		}
	})
	operations := 0
	for _, selected := range []bool{*version, *check, *checkProfiles, *installed != "", *credentialStatus != "", *removeCredential != "", *checkIdentity != ""} {
		if selected {
			operations++
		}
	}
	if operations > 1 {
		log.Fatal("choose only one CLI operation")
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
