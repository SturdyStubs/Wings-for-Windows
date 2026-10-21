package main

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"log"
	"os"
	"sync"
	"time"
	"wings-windows/internal/config"
)

const eventSource = "WingsWindowsNative"

type serviceEvents interface {
	Info(uint32, string) error
	Error(uint32, string) error
	Close() error
}

func openServiceEvents() (serviceEvents, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\EventLog\Application\`+eventSource, registry.QUERY_VALUE)
	if err != nil {
		return nil, err
	}
	managed, _, checkErr := key.GetIntegerValue("WingsWindowsManaged")
	key.Close()
	if checkErr != nil || managed != 1 {
		return nil, fmt.Errorf("unmanaged Event Log source")
	}
	return eventlog.Open(eventSource)
}

type handler struct {
	path       string
	runner     func(context.Context, string, string, func()) error
	openEvents func() (serviceEvents, error)
}

func (h handler) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending, CheckPoint: 1, WaitHint: 30000}
	open := h.openEvents
	if open == nil {
		open = openServiceEvents
	}
	events, err := open()
	if err != nil {
		log.Print("Windows Native Event Log unavailable; register its Application source before starting the service")
	}
	if events != nil {
		defer events.Close()
	}
	write := func(id uint32, message string, failure bool) {
		if events == nil {
			return
		}
		var err error
		message = fmt.Sprintf("version=%s pid=%d %s", config.Version, os.Getpid(), message)
		if failure {
			err = events.Error(id, message)
		} else {
			err = events.Info(id, message)
		}
		if err != nil {
			log.Print("Windows Native Event Log write failed")
		}
	}
	write(100, "Windows Native service starting", false)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	runner := h.runner
	if runner == nil {
		runner = runWithReady
	}
	ready := make(chan struct{})
	var once sync.Once
	go func() { done <- runner(ctx, h.path, "", func() { once.Do(func() { close(ready) }) }) }()
	current := svc.Status{State: svc.StartPending, CheckPoint: 1, WaitHint: 30000}
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	stopping := false
	for {
		select {
		case <-ready:
			ready = nil
			if !stopping {
				current = svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
				status <- current
				write(101, "Windows Native API listener ready", false)
			}
		case <-tick.C:
			if current.State == svc.StartPending || current.State == svc.StopPending {
				current.CheckPoint++
				status <- current
			}
		case err := <-done:
			if err != nil {
				if stopping && errors.Is(err, context.Canceled) {
					write(103, "Windows Native service stopped", false)
					return false, 0
				}
				stage := "runtime"
				var failure *runtimeFailure
				if errors.As(err, &failure) {
					switch failure.stage {
					case "configuration", "panel_sync", "tls", "listener", "http", "shutdown":
						stage = failure.stage
					}
				}
				write(200, fmt.Sprintf("Windows Native service failed; stage=%s. Use local configuration/profile checks and panel/port/TLS diagnostics; raw errors are excluded from Event Log.", stage), true)
				log.Print("Windows Native service failed; stage=" + stage)
				return false, 1
			}
			write(103, "Windows Native service stopped", false)
			return false, 0
		case request, ok := <-requests:
			if !ok {
				requests = nil
				request.Cmd = svc.Stop
			}
			switch request.Cmd {
			case svc.Interrogate:
				status <- current
			case svc.Stop, svc.Shutdown:
				if !stopping {
					write(102, "Windows Native service stop requested", false)
				}
				stopping = true
				current = svc.Status{State: svc.StopPending, CheckPoint: 1, WaitHint: 30000}
				status <- current
				cancel()
			}
		}
	}
}
func serviceMain(path, installed string) bool {
	isService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatal(err)
	}
	if !isService {
		return false
	}
	if installed != "" {
		log.Fatal("mark-installed cannot be used as a Windows service")
	}
	if err = svc.Run("WingsWindowsNative", handler{path: path}); err != nil {
		log.Fatal(err)
	}
	return true
}
