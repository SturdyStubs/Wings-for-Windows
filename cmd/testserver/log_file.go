package main

import (
	"fmt"
	"os"
)

func writeFixtureLog(command string) bool {
	var text string
	switch command {
	case "log-ready":
		text = "SERVER_READY\n"
	case "log-message":
		text = "FILE_MESSAGE\n"
	case "log-rotate":
		if err := os.Rename("server.log", "server.log.1"); err != nil {
			fmt.Println("LOG_WRITE_ERROR")
			return true
		}
		text = "ROTATED_MESSAGE\n"
	case "log-truncate":
		if err := os.WriteFile("server.log", []byte("RESET\n"), 0600); err != nil {
			fmt.Println("LOG_WRITE_ERROR")
		}
		return true
	case "log-malformed":
		text = "\x00BROKEN\n"
	default:
		return false
	}
	file, err := os.OpenFile("server.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Println("LOG_WRITE_ERROR")
		return true
	}
	_, err = file.WriteString(text)
	file.Close()
	if err != nil {
		fmt.Println("LOG_WRITE_ERROR")
	}
	return true
}
