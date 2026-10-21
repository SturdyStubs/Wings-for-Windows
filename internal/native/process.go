package native

import (
	"context"
	"errors"
	"io"
	"os"
	"time"
	"wings-windows/internal/identity"
	"wings-windows/internal/networkpolicy"
)

type Spec struct {
	NetworkPolicy          networkpolicy.Policy
	Context                context.Context
	RunAs                  *identity.Reference
	AccessRules            []identity.PathRule
	Executable             string
	Arguments, Environment []string
	Directory              string
	TempDirectory          string
	RequiredFiles          []string
	MemoryMB, CPUPercent   int
}
type Stats struct {
	CPUTime100ns uint64
	MemoryBytes  uint64
}
type Limits struct{ MemoryMB, CPUPercent int }

var ErrExited = errors.New("native process exited")

func (p *Process) UpdateLimits(limits Limits) error {
	if limits.MemoryMB < 0 || limits.MemoryMB > 1<<30 || limits.CPUPercent < 0 || limits.CPUPercent > 1_000_000 {
		return errors.New("invalid native process limits")
	}
	return p.implementation.updateLimits(limits)
}

type Process struct {
	CleanupError   error // Read only after Done closes.
	implementation platformProcess
	Stdin          io.WriteCloser
	Stdout         io.ReadCloser
	Done           chan struct{}
	ExitCode       uint32
	input          chan inputRequest
}
type inputRequest struct {
	text   string
	result chan error
}

func Start(spec Spec) (*Process, error) {
	if err := spec.contextError(); err != nil {
		return nil, err
	}
	if spec.Executable == "" || spec.Directory == "" {
		return nil, errors.New("executable and directory required")
	}
	if info, err := os.Stat(spec.Executable); err != nil {
		return nil, err
	} else if !info.Mode().IsRegular() {
		return nil, errors.New("executable must be a regular file")
	}
	p, err := startPlatform(spec)
	if err != nil {
		return nil, err
	}
	p.input = make(chan inputRequest, 8)
	go func() {
		for {
			select {
			case <-p.Done:
				return
			case request := <-p.input:
				_, err := io.WriteString(p.Stdin, request.text+"\r\n")
				request.result <- err
			}
		}
	}()
	return p, nil
}

func (s Spec) contextError() error {
	if s.Context != nil {
		return s.Context.Err()
	}
	return nil
}
func (p *Process) Command(text string) error {
	request := inputRequest{text, make(chan error, 1)}
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case <-p.Done:
		return errors.New("process exited")
	case p.input <- request:
	case <-timer.C:
		return errors.New("console input queue is full")
	}
	select {
	case err := <-request.result:
		return err
	case <-p.Done:
		return errors.New("process exited")
	case <-timer.C:
		return errors.New("console input timed out; process is not reading stdin")
	}
}
func (p *Process) Kill() error           { return p.implementation.kill() }
func (p *Process) Stats() (Stats, error) { return p.implementation.stats() }
