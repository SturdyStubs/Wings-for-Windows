package native

import (
	"golang.org/x/sys/windows"
	"testing"
	"unsafe"
)

func TestLiveJobLimits(t *testing.T) {
	p := launch(t, "")
	out := lines(p)
	line(t, out)
	for _, desired := range []Limits{{192, 50}, {256, 200}, {0, 0}, {128, 100}} {
		if err := p.UpdateLimits(desired); err != nil {
			t.Fatalf("update %+v: %v", desired, err)
		}
		state := p.implementation.state
		state.Lock()
		var memory windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
		err := windows.QueryInformationJobObject(state.job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&memory)), uint32(unsafe.Sizeof(memory)), nil)
		var cpu jobCPU
		cpuErr := windows.QueryInformationJobObject(state.job, 15, uintptr(unsafe.Pointer(&cpu)), uint32(unsafe.Sizeof(cpu)), nil)
		state.Unlock()
		if err != nil || cpuErr != nil {
			t.Fatal(err, cpuErr)
		}
		if memory.JobMemoryLimit != uintptr(desired.MemoryMB)*1024*1024 {
			t.Fatalf("OS memory limit mismatch: %d", memory.JobMemoryLimit)
		}
		if memory.BasicLimitInformation.LimitFlags&windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE == 0 {
			t.Fatal("live update removed kill-on-close")
		}
		if desired.CPUPercent == 0 && cpu.Flags != 0 {
			t.Fatal("CPU control not disabled", cpu)
		}
		if desired.CPUPercent > 0 && (cpu.Flags&5 != 5 || cpu.Rate == 0) {
			t.Fatal("CPU control not applied", cpu)
		}
	}
	if err := p.UpdateLimits(Limits{-1, 100}); err == nil {
		t.Fatal("invalid limits accepted")
	}
	if err := p.Command("still alive"); err != nil {
		t.Fatal(err)
	}
	if got := line(t, out); got != "ECHO:still alive" {
		t.Fatal(got)
	}
	p.Kill()
	<-p.Done
	if err := p.UpdateLimits(Limits{128, 100}); err != ErrExited {
		t.Fatal("exited update", err)
	}
}
