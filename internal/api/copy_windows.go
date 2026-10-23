package api

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"unsafe"
)

func openCopyHandle(parent *os.File, leaf string, create bool) (*os.File, error) {
	if !validRenameLeaf(leaf) {
		return nil, windows.ERROR_INVALID_NAME
	}
	name, err := windows.NewNTUnicodeString(leaf)
	if err != nil {
		return nil, err
	}
	attrs := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(parent.Fd()), ObjectName: name, Attributes: windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE}
	attrs.Length = uint32(unsafe.Sizeof(attrs))
	access, disposition, share := uint32(windows.FILE_GENERIC_READ), uint32(windows.FILE_OPEN), uint32(windows.FILE_SHARE_READ)
	if create {
		access = windows.FILE_GENERIC_WRITE | windows.FILE_READ_ATTRIBUTES | windows.DELETE
		disposition = windows.FILE_CREATE
		share = 0
	}
	var h windows.Handle
	err = windows.NtCreateFile(&h, access, &attrs, &windows.IO_STATUS_BLOCK{}, nil, windows.FILE_ATTRIBUTE_NORMAL, share, disposition, windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	if err != nil {
		return nil, renameWindowsError(err)
	}
	return os.NewFile(uintptr(h), leaf), nil
}

func discardCopyStage(f *os.File) error {
	info := struct{ DeleteFile byte }{1}
	return renameWindowsError(windows.NtSetInformationFile(windows.Handle(f.Fd()), &windows.IO_STATUS_BLOCK{}, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), windows.FileDispositionInformation))
}

func copyDiskFull(err error) bool {
	return errors.Is(err, windows.ERROR_DISK_FULL) || errors.Is(err, windows.ERROR_HANDLE_DISK_FULL)
}
