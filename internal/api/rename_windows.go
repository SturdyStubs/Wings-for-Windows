package api

import (
	"golang.org/x/sys/windows"
	"os"
	"unsafe"
)

// Use a leaf name relative to a checked directory, never an absolute pathname.
// No delete sharing pins the exact source against ordinary replacement.
func openRenameSource(parent *os.File, leaf string) (*os.File, error) {
	if !validRenameLeaf(leaf) {
		return nil, windows.ERROR_INVALID_NAME
	}
	name, err := windows.NewNTUnicodeString(leaf)
	if err != nil {
		return nil, err
	}
	attrs := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(parent.Fd()), ObjectName: name, Attributes: windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE}
	attrs.Length = uint32(unsafe.Sizeof(attrs))
	var h windows.Handle
	err = windows.NtCreateFile(&h, windows.DELETE|windows.FILE_READ_DATA|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE, &attrs, &windows.IO_STATUS_BLOCK{}, nil, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, windows.FILE_OPEN, windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	if err != nil {
		return nil, renameWindowsError(err)
	}
	return os.NewFile(uintptr(h), leaf), nil
}

func inspectRenameEntry(f *os.File) error {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &info); err != nil {
		return err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 && info.NumberOfLinks != 1 {
		return &directoryFailure{403, "Rename refuses links, reparse points and multi-hardlink files"}
	}
	return nil
}

// Native FileRenameInformation with ReplaceIfExists=false enforces collision
// protection at the actual move, including a destination created after preflight.
// Structure layout follows Windows FILE_RENAME_INFORMATION and Go's Renameat.
func renameNoReplace(source, destinationParent *os.File, leaf string) error {
	if !validRenameLeaf(leaf) {
		return windows.ERROR_INVALID_NAME
	}
	name, err := windows.UTF16FromString(leaf)
	if err != nil || len(name) > 256 {
		return windows.ERROR_INVALID_NAME
	}
	var info struct {
		ReplaceIfExists uint32
		RootDirectory   windows.Handle
		FileNameLength  uint32
		FileName        [256]uint16
	}
	info.RootDirectory = windows.Handle(destinationParent.Fd())
	info.FileNameLength = uint32((len(name) - 1) * 2)
	copy(info.FileName[:], name)
	err = windows.NtSetInformationFile(windows.Handle(source.Fd()), &windows.IO_STATUS_BLOCK{}, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), windows.FileRenameInformation)
	return renameWindowsError(err)
}

func renameWindowsError(err error) error {
	if status, ok := err.(windows.NTStatus); ok {
		return status.Errno()
	}
	return err
}
