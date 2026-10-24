package diskusage

import (
	"encoding/binary"
	"golang.org/x/sys/windows"
	"os"
)

// Mount-point junctions exercise Windows reparse-point confinement without
// requiring the symbolic-link privilege or enabling Developer Mode.
func createTestLink(target, link string) error {
	if err := os.Mkdir(link, 0700); err != nil {
		return err
	}
	name, err := windows.UTF16PtrFromString(link)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	substitute, err := windows.UTF16FromString(`\??\` + target)
	if err != nil {
		return err
	}
	printName, err := windows.UTF16FromString(target)
	if err != nil {
		return err
	}
	buffer := make([]byte, 16+2*(len(substitute)+len(printName)))
	binary.LittleEndian.PutUint32(buffer, windows.IO_REPARSE_TAG_MOUNT_POINT)
	binary.LittleEndian.PutUint16(buffer[4:], uint16(len(buffer)-8))
	binary.LittleEndian.PutUint16(buffer[10:], uint16((len(substitute)-1)*2))
	binary.LittleEndian.PutUint16(buffer[12:], uint16(len(substitute)*2))
	binary.LittleEndian.PutUint16(buffer[14:], uint16((len(printName)-1)*2))
	for i, v := range append(substitute, printName...) {
		binary.LittleEndian.PutUint16(buffer[16+i*2:], v)
	}
	var returned uint32
	return windows.DeviceIoControl(h, windows.FSCTL_SET_REPARSE_POINT, &buffer[0], uint32(len(buffer)), nil, 0, &returned, nil)
}
