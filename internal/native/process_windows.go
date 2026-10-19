package native

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

type processState struct {
	sync.Mutex
	job, process windows.Handle
}
type platformProcess struct{ state *processState }

func startPlatform(spec Spec) (result *Process, err error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			windows.CloseHandle(job)
		}
	}()
	if err = setJobLimits(job, Limits{spec.MemoryMB, spec.CPUPercent}); err != nil {
		return nil, err
	}
	sa := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), InheritHandle: 1}
	var inRead, inWrite, outRead, outWrite windows.Handle
	if err = windows.CreatePipe(&inRead, &inWrite, &sa, 0); err != nil {
		return nil, err
	}
	defer windows.CloseHandle(inRead)
	if err = windows.CreatePipe(&outRead, &outWrite, &sa, 0); err != nil {
		windows.CloseHandle(inWrite)
		return nil, err
	}
	defer windows.CloseHandle(outWrite)
	defer func() {
		if err != nil {
			windows.CloseHandle(inWrite)
			windows.CloseHandle(outRead)
		}
	}()
	if err = windows.SetHandleInformation(inWrite, windows.HANDLE_FLAG_INHERIT, 0); err != nil {
		return nil, err
	}
	if err = windows.SetHandleInformation(outRead, windows.HANDLE_FLAG_INHERIT, 0); err != nil {
		return nil, err
	}
	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return nil, err
	}
	defer attrs.Delete()
	handles := []windows.Handle{inRead, outWrite}
	if err = attrs.Update(windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST, unsafe.Pointer(&handles[0]), uintptr(len(handles))*unsafe.Sizeof(handles[0])); err != nil {
		return nil, err
	}
	si := windows.StartupInfoEx{StartupInfo: windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfoEx{})), Flags: windows.STARTF_USESTDHANDLES, StdInput: inRead, StdOutput: outWrite, StdErr: outWrite}, ProcThreadAttributeList: attrs.List()}
	app, err := windows.UTF16PtrFromString(spec.Executable)
	if err != nil {
		return nil, err
	}
	dir, err := windows.UTF16PtrFromString(spec.Directory)
	if err != nil {
		return nil, err
	}
	args := []string{syscall.EscapeArg(spec.Executable)}
	for _, arg := range spec.Arguments {
		if strings.ContainsRune(arg, 0) {
			return nil, fmt.Errorf("NUL in argument")
		}
		args = append(args, syscall.EscapeArg(arg))
	}
	line, err := windows.UTF16PtrFromString(strings.Join(args, " "))
	if err != nil {
		return nil, err
	}
	env := append([]string{}, spec.Environment...)
	sort.Slice(env, func(i, j int) bool { return strings.ToUpper(env[i]) < strings.ToUpper(env[j]) })
	block := []uint16{}
	for _, entry := range env {
		v, e := windows.UTF16FromString(entry)
		if e != nil {
			return nil, e
		}
		block = append(block, v...)
	}
	block = append(block, 0)
	if len(block) == 1 {
		block = append(block, 0)
	}
	var pi windows.ProcessInformation
	if err = windows.CreateProcess(app, line, nil, nil, true, windows.CREATE_SUSPENDED|windows.CREATE_UNICODE_ENVIRONMENT|windows.EXTENDED_STARTUPINFO_PRESENT|windows.CREATE_NO_WINDOW, &block[0], dir, &si.StartupInfo, &pi); err != nil {
		return nil, err
	}
	defer windows.CloseHandle(pi.Thread)
	if err = windows.AssignProcessToJobObject(job, pi.Process); err != nil {
		windows.TerminateProcess(pi.Process, 1)
		windows.CloseHandle(pi.Process)
		return nil, err
	}
	if _, err = windows.ResumeThread(pi.Thread); err != nil {
		windows.TerminateJobObject(job, 1)
		windows.CloseHandle(pi.Process)
		return nil, err
	}
	state := &processState{job: job, process: pi.Process}
	p := &Process{implementation: platformProcess{state}, Stdin: os.NewFile(uintptr(inWrite), "stdin"), Stdout: os.NewFile(uintptr(outRead), "stdout"), Done: make(chan struct{})}
	go func() {
		windows.WaitForSingleObject(pi.Process, windows.INFINITE)
		state.Lock()
		windows.GetExitCodeProcess(pi.Process, &p.ExitCode)
		windows.CloseHandle(job)
		windows.CloseHandle(pi.Process)
		state.job = 0
		state.process = 0
		state.Unlock()
		p.Stdin.Close()
		close(p.Done)
	}()
	return p, nil
}

type jobCPU struct{ Flags, Rate uint32 }

func setJobLimits(job windows.Handle, desired Limits) error {
	if desired.MemoryMB < 0 || desired.MemoryMB > 1<<30 || desired.CPUPercent < 0 || desired.CPUPercent > 1_000_000 {
		return fmt.Errorf("invalid job limits")
	}
	var before windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	var previousCPU jobCPU
	if err := windows.QueryInformationJobObject(job, 15, uintptr(unsafe.Pointer(&previousCPU)), uint32(unsafe.Sizeof(previousCPU)), nil); err != nil {
		return err
	}
	if err := windows.QueryInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&before)), uint32(unsafe.Sizeof(before)), nil); err != nil {
		return err
	}
	limits := before
	limits.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	limits.BasicLimitInformation.LimitFlags &^= windows.JOB_OBJECT_LIMIT_JOB_MEMORY
	limits.JobMemoryLimit = 0
	if desired.MemoryMB > 0 {
		limits.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_JOB_MEMORY
		limits.JobMemoryLimit = uintptr(desired.MemoryMB) * 1024 * 1024
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return err
	}
	if desired.CPUPercent == 0 && previousCPU.Flags == 0 {
		return nil
	}
	// Windows still validates CpuRate when disabling an existing cap. Supply
	// an in-range value with the enable flag cleared, rather than CpuRate=0.
	cpu := jobCPU{Rate: 10000}
	if desired.CPUPercent > 0 {
		rate := desired.CPUPercent * 100 / runtime.NumCPU()
		if rate < 1 {
			rate = 1
		}
		if rate > 10000 {
			rate = 10000
		}
		cpu = jobCPU{5, uint32(rate)}
	}
	if _, err := windows.SetInformationJobObject(job, 15, uintptr(unsafe.Pointer(&cpu)), uint32(unsafe.Sizeof(cpu))); err != nil {
		if _, rollbackErr := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&before)), uint32(unsafe.Sizeof(before))); rollbackErr != nil {
			windows.TerminateJobObject(job, 1)
			return fmt.Errorf("CPU limit update failed; memory rollback failed and job was terminated: %w", rollbackErr)
		}
		return err
	}
	return nil
}
func (p platformProcess) updateLimits(limits Limits) error {
	p.state.Lock()
	defer p.state.Unlock()
	if p.state.job == 0 {
		return ErrExited
	}
	return setJobLimits(p.state.job, limits)
}
func (p platformProcess) kill() error {
	p.state.Lock()
	defer p.state.Unlock()
	if p.state.job == 0 {
		return nil
	}
	return windows.TerminateJobObject(p.state.job, 1)
}
func (p platformProcess) stats() (Stats, error) {
	p.state.Lock()
	defer p.state.Unlock()
	if p.state.process == 0 {
		return Stats{}, fmt.Errorf("process exited")
	}
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(p.state.process, &created, &exited, &kernel, &user); err != nil {
		return Stats{}, err
	}
	ticks := func(f windows.Filetime) uint64 { return uint64(f.HighDateTime)<<32 | uint64(f.LowDateTime) }
	var memory struct {
		Size, PageFaults                                                                                      uint32
		PeakWorkingSet, WorkingSet, PeakPaged, Paged, PeakNonpaged, Nonpaged, Pagefile, PeakPagefile, Private uintptr
	}
	memory.Size = uint32(unsafe.Sizeof(memory))
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("K32GetProcessMemoryInfo")
	if ok, _, err := proc.Call(uintptr(p.state.process), uintptr(unsafe.Pointer(&memory)), uintptr(memory.Size)); ok == 0 {
		return Stats{}, err
	}
	return Stats{CPUTime100ns: ticks(kernel) + ticks(user), MemoryBytes: uint64(memory.WorkingSet)}, nil
}
