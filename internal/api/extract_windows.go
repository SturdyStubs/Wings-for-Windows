package api

import (
	"golang.org/x/sys/windows"
	"os"
	"unsafe"
)

// Create exactly one new directory relative to an already checked parent.
// FILE_CREATE refuses collisions; the returned handle also owns cleanup/child
// creation without reopening an arbitrary pathname.
// Allow ordinary directory reads/child operations but no delete sharing: this
// admits native child publication while pinning the new directory against moves.
func createExtractDirectory(parent *os.File, leaf string) (*os.File, error) {
	if !validRenameLeaf(leaf) {
		return nil, windows.ERROR_INVALID_NAME
	}
	name, e := windows.NewNTUnicodeString(leaf)
	if e != nil {
		return nil, e
	}
	attrs := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(parent.Fd()), ObjectName: name, Attributes: windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE}
	attrs.Length = uint32(unsafe.Sizeof(attrs))
	var h windows.Handle
	e = windows.NtCreateFile(&h, windows.FILE_GENERIC_READ|windows.FILE_GENERIC_WRITE|windows.DELETE, &attrs, &windows.IO_STATUS_BLOCK{}, nil, windows.FILE_ATTRIBUTE_DIRECTORY, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, windows.FILE_CREATE, windows.FILE_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	if e != nil {
		return nil, renameWindowsError(e)
	}
	return os.NewFile(uintptr(h), leaf), nil
}
