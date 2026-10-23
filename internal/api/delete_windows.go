package api

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"unsafe"
)

// Mark only this validated handle for deletion; close commits ordinary NTFS
// deletion. Do not change read-only attributes or bypass sharing permissions.
func deletePinnedEntry(f *os.File) error {
	info := struct{ DeleteFile byte }{1}
	return renameWindowsError(windows.NtSetInformationFile(windows.Handle(f.Fd()), &windows.IO_STATUS_BLOCK{}, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), windows.FileDispositionInformation))
}
func deleteWindowsStatus(err error) int {
	if errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return 423
	}
	if errors.Is(err, windows.ERROR_DIR_NOT_EMPTY) {
		return 409
	}
	return 0
}
