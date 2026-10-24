package native

import (
	"fmt"
	"golang.org/x/sys/windows"
	"io"
	"os"
	"sync"
	"unsafe"
)

const terminalBufferBytes = 256 << 10

// Output is drained independently of its consumer, including during HPCON
// shutdown on Server 2022, where ClosePseudoConsole can block on a full pipe.
type terminalOutput struct {
	mu                          sync.Mutex
	cond                        *sync.Cond
	data                        []byte
	truncated, finished, closed bool
}

func newTerminalOutput() *terminalOutput {
	o := &terminalOutput{}
	o.cond = sync.NewCond(&o.mu)
	return o
}
func (o *terminalOutput) append(p []byte) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return
	}
	if len(p) > terminalBufferBytes {
		p = p[len(p)-terminalBufferBytes:]
		o.truncated = true
	}
	excess := len(o.data) + len(p) - terminalBufferBytes
	if excess > 0 {
		o.data = append(o.data[:0], o.data[excess:]...)
		o.truncated = true
	}
	o.data = append(o.data, p...)
	o.cond.Broadcast()
}
func (o *terminalOutput) finish() { o.mu.Lock(); o.finished = true; o.cond.Broadcast(); o.mu.Unlock() }
func (o *terminalOutput) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	for len(o.data) == 0 && !o.finished && !o.closed && !o.truncated {
		o.cond.Wait()
	}
	if o.closed {
		return 0, io.EOF
	}
	if o.truncated {
		o.truncated = false
		o.data = append([]byte("\r\n[Windows node] pseudoconsole output truncated\r\n"), o.data...)
	}
	if len(o.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, o.data)
	o.data = o.data[n:]
	return n, nil
}
func (o *terminalOutput) Close() error {
	o.mu.Lock()
	o.closed = true
	o.data = nil
	o.cond.Broadcast()
	o.mu.Unlock()
	return nil
}

type pseudoTerminal struct {
	handle windows.Handle
	output *terminalOutput
	once   sync.Once
}

func newPseudoTerminal(input, output, read windows.Handle) (*pseudoTerminal, error) {
	if e := windows.NewLazySystemDLL("kernel32.dll").NewProc("CreatePseudoConsole").Find(); e != nil {
		return nil, e
	}
	var h windows.Handle
	if e := windows.CreatePseudoConsole(windows.Coord{X: 120, Y: 40}, input, output, 0, &h); e != nil {
		return nil, fmt.Errorf("create pseudoconsole: %w", e)
	}
	p := &pseudoTerminal{handle: h, output: newTerminalOutput()}
	f := os.NewFile(uintptr(read), "pseudoconsole-output")
	go func() {
		defer f.Close()
		defer p.output.finish()
		buf := make([]byte, 16<<10)
		for {
			n, e := f.Read(buf)
			if n > 0 {
				p.output.append(buf[:n])
			}
			if e != nil {
				return
			}
		}
	}()
	return p, nil
}

var updateTerminalAttribute = windows.NewLazySystemDLL("kernel32.dll").NewProc("UpdateProcThreadAttribute")

func (p *pseudoTerminal) attach(attrs *windows.ProcThreadAttributeListContainer) error {
	// This attribute takes the HPCON value itself, unlike HANDLE_LIST, which
	// takes an array pointer. Keep the opaque handle out of Go pointer storage.
	r, _, e := updateTerminalAttribute.Call(uintptr(unsafe.Pointer(attrs.List())), 0, windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, uintptr(p.handle), unsafe.Sizeof(p.handle), 0, 0)
	if r == 0 {
		return e
	}
	return nil
}
func (p *pseudoTerminal) close() { p.once.Do(func() { windows.ClosePseudoConsole(p.handle) }) }
