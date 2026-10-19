//go:build !windows

package native

import "errors"

func HostInfo(string) (HostInformation, error) {
	return HostInformation{}, errors.New("host information requires Windows")
}
