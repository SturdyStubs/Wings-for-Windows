package native

import (
	"fmt"
	"golang.org/x/sys/windows"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
	"wings-windows/internal/identity"
	"wings-windows/internal/networkpolicy"
)

type processState struct {
	sync.Mutex
	job, process windows.Handle
}
type platformProcess struct{ state *processState }

func startPlatform(spec Spec) (result *Process, err error) {
	if err := spec.NetworkPolicy.Validate(spec.RunAs); err != nil {
		return nil, err
	}
	var token windows.Token
	if spec.RunAs != nil {
		token, err = identity.Open(*spec.RunAs)
		if err != nil {
			return nil, err
		}
		defer token.Close()
		if err = identity.CheckDaemonAccess(token); err != nil {
			return nil, err
		}
		paths := []string{spec.Executable, spec.Directory, spec.TempDirectory}
		paths = append(paths, spec.RequiredFiles...)
		for _, rule := range spec.AccessRules {
			// Recursive other-server data stays a bounded snapshot audit;
			// retaining one handle per data file would exhaust node resources.
			if !rule.Recursive {
				paths = append(paths, rule.Path)
			}
		}
		guard, guardErr := identity.PinLaunchPaths(spec.Context, paths)
		if guardErr != nil {
			return nil, guardErr
		}
		defer guard.Close()
		if err = identity.CheckAccessContext(spec.Context, token, spec.AccessRules); err != nil {
			return nil, err
		}
		if err = identity.CheckLaunchAccessContext(spec.Context, token, identity.LaunchAccess{Executable: spec.Executable, Directory: spec.Directory, TempDirectory: spec.TempDirectory, RequiredFiles: spec.RequiredFiles}); err != nil {
			return nil, err
		}
	}
	policy, err := networkpolicy.Install(spec.Context, spec.NetworkPolicy, spec.RunAs)
	if err != nil {
		return nil, err
	}
	p, err := startWithTokenAndPolicy(spec, token, policy)
	if err != nil && policy != nil {
		if cleanupErr := policy.Close(); cleanupErr != nil {
			return nil, fmt.Errorf("launch failed (%v); network rollback failed: %w", err, cleanupErr)
		}
	}
	return p, err
}

func startWithToken(spec Spec, token windows.Token) (result *Process, err error) {
	return startWithTokenAndPolicy(spec, token, nil)
}

func startWithTokenAndPolicy(spec Spec, token windows.Token, policy io.Closer) (result *Process, err error) {
	if err := spec.contextError(); err != nil {
		return nil, err
	}
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
	if err = spec.contextError(); err != nil {
		return nil, err
	}
	flags := uint32(windows.CREATE_SUSPENDED | windows.CREATE_UNICODE_ENVIRONMENT | windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_NO_WINDOW)
	if token != 0 {
		err = windows.CreateProcessAsUser(token, app, line, nil, nil, true, flags, &block[0], dir, &si.StartupInfo, &pi)
	} else {
		err = windows.CreateProcess(app, line, nil, nil, true, flags, &block[0], dir, &si.StartupInfo, &pi)
	}
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(pi.Thread)
	if err = windows.AssignProcessToJobObject(job, pi.Process); err != nil {
		windows.TerminateProcess(pi.Process, 1)
		windows.CloseHandle(pi.Process)
		return nil, err
	}
	if err = spec.contextError(); err != nil {
		windows.TerminateJobObject(job, 1)
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
		p.CleanupError = finishJobPolicy(job, policy)
		windows.CloseHandle(pi.Process)
		state.job = 0
		state.process = 0
		state.Unlock()
		p.Stdin.Close()
		close(p.Done)
	}()
	return p, nil
}

var retainedPolicies struct {
	sync.Mutex
	policies []io.Closer
}

func finishJobPolicy(job windows.Handle, policy io.Closer) error {
	if policy == nil {
		return windows.CloseHandle(job)
	}
	// Keep the block active until all descendants, not just the root, exit.
	handles, err := jobProcessHandles(job)
	defer func() {
		for _, handle := range handles {
			windows.CloseHandle(handle)
		}
	}()
	if killErr := windows.TerminateJobObject(job, 1); err == nil {
		err = killErr
	}
	deadline := time.Now().Add(5 * time.Second)
	for err == nil {
		var info struct {
			User, Kernel, PeriodUser, PeriodKernel int64
			PageFaults, Total, Active, Terminated  uint32
		}
		err = windows.QueryInformationJobObject(job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), nil)
		if err != nil || info.Active == 0 {
			break
		}
		if time.Now().After(deadline) {
			err = fmt.Errorf("job descendants did not drain before network cleanup deadline")
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err == nil {
		for _, handle := range handles {
			remaining := time.Until(deadline).Milliseconds()
			if remaining < 0 {
				remaining = 0
			}
			status, waitErr := windows.WaitForSingleObject(handle, uint32(remaining))
			if waitErr != nil {
				err = waitErr
				break
			}
			if status != windows.WAIT_OBJECT_0 {
				err = fmt.Errorf("job process did not finish terminating before network cleanup deadline")
				break
			}
		}
	}
	windows.CloseHandle(job)
	if err != nil {
		retainedPolicies.Lock()
		retainedPolicies.policies = append(retainedPolicies.policies, policy)
		retainedPolicies.Unlock()
		return fmt.Errorf("network block retained until daemon exit because job cleanup failed: %w", err)
	}
	return policy.Close()
}

func jobProcessHandles(job windows.Handle) ([]windows.Handle, error) {
	var list struct {
		Assigned, Count uint32
		IDs             [4096]uintptr
	}
	if err := windows.QueryInformationJobObject(job, windows.JobObjectBasicProcessIdList, uintptr(unsafe.Pointer(&list)), uint32(unsafe.Sizeof(list)), nil); err != nil {
		return nil, err
	}
	if list.Count > uint32(len(list.IDs)) || list.Assigned > uint32(len(list.IDs)) {
		return nil, fmt.Errorf("job exceeds network cleanup process audit bound")
	}
	var handles []windows.Handle
	for _, pid := range list.IDs[:list.Count] {
		handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
		if err == windows.ERROR_INVALID_PARAMETER {
			continue
		}
		if err != nil {
			return handles, err
		}
		handles = append(handles, handle)
	}
	return handles, nil
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
