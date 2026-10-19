package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
	"wings-windows/internal/api"
	"wings-windows/internal/config"
	"wings-windows/internal/node"
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
		if info, statErr := os.Stat(s.Profile.Executable); statErr != nil || !info.Mode().IsRegular() {
			err = statErr
			if err == nil {
				err = fmt.Errorf("not a regular file")
			}
			return fmt.Errorf("executable must be provisioned: %w", err)
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
	version := flag.Bool("version", false, "Print version")
	installed := flag.String("mark-installed", "", "Explicitly acknowledge manually provisioned server UUID to the panel")
	flag.Parse()
	if *version {
		fmt.Println(config.Version)
		return
	}
	if *check {
		if _, err := config.Load(*path); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Configuration valid")
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
