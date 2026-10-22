package main

// Independent harmless wire fixture, deliberately not the production client.
import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

const rconFixturePassword = "fixture-rcon-only"

func rconRead(c net.Conn) (id, kind int32, body string, err error) {
	var n uint32
	if err = binary.Read(c, binary.LittleEndian, &n); err != nil {
		return
	}
	if n < 10 || n > 4096 {
		err = fmt.Errorf("bad frame")
		return
	}
	b := make([]byte, n)
	if _, err = io.ReadFull(c, b); err != nil {
		return
	}
	if b[len(b)-1] != 0 || b[len(b)-2] != 0 {
		err = fmt.Errorf("bad padding")
		return
	}
	id = int32(binary.LittleEndian.Uint32(b))
	kind = int32(binary.LittleEndian.Uint32(b[4:]))
	body = string(b[8 : len(b)-2])
	return
}
func rconWrite(c net.Conn, id, kind int32, body string) {
	binary.Write(c, binary.LittleEndian, int32(len(body)+10))
	binary.Write(c, binary.LittleEndian, id)
	binary.Write(c, binary.LittleEndian, kind)
	io.WriteString(c, body+"\x00\x00")
}
func startRCONFixture(address string) {
	if !strings.HasPrefix(address, "127.0.0.1:") {
		panic("fixture loopback required")
	}
	l, err := net.Listen("tcp", address)
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				id, kind, password, err := rconRead(c)
				if err != nil || kind != 3 {
					return
				}
				rconWrite(c, id, 0, "")
				if password != rconFixturePassword {
					rconWrite(c, -1, 2, "")
					return
				}
				rconWrite(c, id, 2, "")
				id, kind, command, err := rconRead(c)
				if err != nil || kind != 2 {
					return
				}
				end, kind, body, err := rconRead(c)
				if err != nil || kind != 0 || body != "" {
					return
				}
				fmt.Println("RCON_COMMAND:" + command)
				probeRCONCredential(command)
				writeFixtureLog(command)
				if command == "hang" {
					io.Copy(io.Discard, c)
					return
				}
				if command == "echo-secret" {
					rconWrite(c, id, 0, rconFixturePassword)
				} else {
					rconWrite(c, id, 0, "ECHO:"+command)
				}
				rconWrite(c, end, 0, "")
				if command == "stop" {
					fmt.Println("STOPPING")
					os.Exit(0)
				}
			}()
		}
	}()
}
