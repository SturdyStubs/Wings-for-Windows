package native

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"runtime"
	"unsafe"
)

func HostInfo(dataDirectory string) (HostInformation, error) {
	info := HostInformation{Architecture: runtime.GOARCH, CPUThreads: runtime.NumCPU(), OSType: "windows", Filesystem: "unavailable"}
	version := windows.RtlGetVersion()
	info.KernelVersion = fmt.Sprintf("%d.%d.%d", version.MajorVersion, version.MinorVersion, version.BuildNumber)
	info.OS = "Microsoft Windows NT " + info.KernelVersion
	if n := windows.GetActiveProcessorCount(0xffff); n > 0 {
		info.CPUThreads = int(n)
	}
	var memory struct {
		Length, Load                                                                                                          uint32
		TotalPhysical, AvailablePhysical, TotalPagefile, AvailablePagefile, TotalVirtual, AvailableVirtual, AvailableExtended uint64
	}
	memory.Length = uint32(unsafe.Sizeof(memory))
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")
	if ok, _, err := proc.Call(uintptr(unsafe.Pointer(&memory))); ok == 0 {
		return info, fmt.Errorf("read physical memory: %w", err)
	}
	info.MemoryBytes = int64(memory.TotalPhysical)
	if f, err := os.Open(dataDirectory); err == nil {
		defer f.Close()
		buffer := make([]uint16, 256)
		if err := windows.GetVolumeInformationByHandle(windows.Handle(f.Fd()), nil, 0, nil, nil, nil, &buffer[0], uint32(len(buffer))); err == nil {
			info.Filesystem = windows.UTF16ToString(buffer)
		}
	}
	return info, nil
}
