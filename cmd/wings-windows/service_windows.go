package main

import (
	"context"
	"golang.org/x/sys/windows/svc"
	"log"
)

type handler struct{ path string }

func (h handler) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, h.path, "") }()
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case err := <-done:
			if err != nil {
				log.Printf("Windows Native stopped: %v", err)
				return false, 1
			}
			return false, 0
		case request := <-requests:
			switch request.Cmd {
			case svc.Interrogate:
				status <- request.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
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
	if err = svc.Run("WingsWindowsNative", handler{path}); err != nil {
		log.Fatal(err)
	}
	return true
}
