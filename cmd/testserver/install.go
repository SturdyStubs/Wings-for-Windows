package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func runInstallerFixture() {
	mode := "success"
	if len(os.Args) > 2 {
		mode = os.Args[2]
	}
	fmt.Println("INSTALL_BEGIN")
	fmt.Println("INSTALL_VALUE:" + os.Getenv("INSTALL_VALUE"))
	child := exec.Command(os.Args[0], "child")
	if child.Start() == nil {
		fmt.Printf("INSTALL_CHILD:%d\n", child.Process.Pid)
	}
	if err := os.WriteFile("install.marker", []byte("installed"), 0600); err != nil {
		os.Exit(24)
	}
	switch mode {
	case "fail":
		os.Exit(23)
	case "hang":
		for {
			time.Sleep(time.Second)
		}
	case "flood":
		for i := 0; i < 1500; i++ {
			fmt.Println(strings.Repeat("x", 1024))
		}
	case "long-line":
		fmt.Println(strings.Repeat("x", 40<<10))
	}
	fmt.Println("INSTALL_DONE")
	os.Exit(0)
}
