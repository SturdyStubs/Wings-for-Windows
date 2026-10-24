package sftpserver

import (
	"errors"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sys/windows"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unsafe"
)

func loadSigner(filename string) (ssh.Signer, error) {
	if !filepath.IsAbs(filename) || filepath.Clean(filename) != filename || len(filepath.VolumeName(filename)) != 2 {
		return nil, errors.New("invalid key path")
	}
	volume := filepath.VolumeName(filename) + `\`
	p, _ := windows.UTF16PtrFromString(volume)
	if windows.GetDriveType(p) != windows.DRIVE_FIXED {
		return nil, os.ErrPermission
	}
	h, e := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if e != nil {
		return nil, e
	}
	files := []*os.File{os.NewFile(uintptr(h), volume)}
	defer func() {
		for i := len(files) - 1; i >= 0; i-- {
			files[i].Close()
		}
	}()
	rel, e := filepath.Rel(volume, filename)
	if e != nil {
		return nil, e
	}
	parts := strings.Split(rel, `\`)
	if len(parts) > 64 {
		return nil, os.ErrPermission
	}
	for i, part := range parts {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "/:\x00") {
			return nil, os.ErrPermission
		}
		name, e := windows.NewNTUnicodeString(part)
		if e != nil {
			return nil, e
		}
		attrs := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(files[len(files)-1].Fd()), ObjectName: name, Attributes: windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE}
		attrs.Length = uint32(unsafe.Sizeof(attrs))
		share := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE)
		if i == len(parts)-1 {
			share = windows.FILE_SHARE_READ
		}
		var child windows.Handle
		e = windows.NtCreateFile(&child, windows.FILE_GENERIC_READ, &attrs, &windows.IO_STATUS_BLOCK{}, nil, windows.FILE_ATTRIBUTE_NORMAL, share, windows.FILE_OPEN, windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
		if e != nil {
			return nil, e
		}
		f := os.NewFile(uintptr(child), part)
		files = append(files, f)
		var info windows.ByHandleFileInformation
		if e = windows.GetFileInformationByHandle(child, &info); e != nil {
			return nil, e
		}
		dir := info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0
		if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || dir != (i < len(parts)-1) || !dir && info.NumberOfLinks != 1 {
			return nil, os.ErrPermission
		}
	}
	b, e := io.ReadAll(io.LimitReader(files[len(files)-1], 16385))
	if e != nil || len(b) > 16384 {
		return nil, os.ErrPermission
	}
	signer, e := ssh.ParsePrivateKey(b)
	if e != nil {
		return nil, os.ErrPermission
	}
	if signer.PublicKey().Type() != ssh.KeyAlgoED25519 {
		return nil, os.ErrPermission
	}
	return signer, nil
}
