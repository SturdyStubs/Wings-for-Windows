package main

import (
	"golang.org/x/sys/windows"
	"os"
	"strings"
)

func runWindowsConsole() {
	input, e := windows.GetStdHandle(windows.STD_INPUT_HANDLE)
	if e != nil {
		os.Exit(10)
	}
	output, e := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if e != nil {
		os.Exit(11)
	}
	var mode uint32
	if windows.GetConsoleMode(input, &mode) != nil {
		os.Exit(12)
	}
	write := func(s string) {
		v, e := windows.UTF16FromString(s)
		if e != nil {
			os.Exit(13)
		}
		var n uint32
		if windows.WriteConsole(output, &v[0], uint32(len(v)-1), &n, nil) != nil {
			os.Exit(14)
		}
	}
	write("WINCON_READY\r\n")
	for {
		var b [1024]uint16
		var n uint32
		if windows.ReadConsole(input, &b[0], uint32(len(b)), &n, nil) != nil {
			os.Exit(15)
		}
		line := strings.TrimSpace(windows.UTF16ToString(b[:n]))
		if line == "stop" {
			write("WINCON_STOPPED\r\n")
			return
		}
		if line != "" {
			write("WINCON_ECHO:" + line + "\r\n")
		}
	}
}
