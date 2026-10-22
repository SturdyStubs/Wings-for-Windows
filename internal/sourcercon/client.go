// Package sourcercon provides a bounded Source TCP RCON exchange. There is no
// reconnect or resend: a failed response does not prove a command was not run.
package sourcercon

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxBody = 4086 // Source's 4096 byte size field includes ID/type/padding.
const MaxResponse = 64 << 10

var ErrProtocol = errors.New("RCON response is invalid or exceeds supported limits")
var ErrTransport = errors.New("RCON exchange failed; command execution may be unconfirmed; no retry was made")
var ErrAuthentication = errors.New("RCON authentication failed")

type packet struct {
	id, kind int32
	body     string
}

func read(r io.Reader) (packet, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return packet{}, ErrTransport
	}
	size := binary.LittleEndian.Uint32(header[:])
	if size < 10 || size > 4096 {
		return packet{}, ErrProtocol
	}
	b := make([]byte, int(size))
	if _, err := io.ReadFull(r, b); err != nil {
		return packet{}, ErrTransport
	}
	if b[len(b)-1] != 0 || b[len(b)-2] != 0 {
		return packet{}, ErrProtocol
	}
	return packet{int32(binary.LittleEndian.Uint32(b)), int32(binary.LittleEndian.Uint32(b[4:])), string(b[8 : len(b)-2])}, nil
}
func write(w io.Writer, p packet) error {
	if len(p.body) > MaxBody || strings.ContainsRune(p.body, 0) {
		return ErrProtocol
	}
	b := make([]byte, len(p.body)+14)
	binary.LittleEndian.PutUint32(b, uint32(len(b)-4))
	binary.LittleEndian.PutUint32(b[4:], uint32(p.id))
	binary.LittleEndian.PutUint32(b[8:], uint32(p.kind))
	copy(b[12:], p.body)
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil || n <= 0 {
			return ErrTransport
		}
		b = b[n:]
	}
	return nil
}

func Execute(ctx context.Context, address, password, command string, timeout time.Duration) (string, error) {
	// Defense in depth: callers cannot turn the adapter into an arbitrary dialer.
	host, _, err := net.SplitHostPort(address)
	if err != nil || (host != "127.0.0.1" && host != "::1") || timeout < time.Millisecond || timeout > 10*time.Second {
		return "", ErrProtocol
	}
	if password == "" || len(password) > MaxBody || strings.ContainsAny(password, "\x00\r\n") || !utf8.ValidString(password) {
		return "", errors.New("RCON password is unavailable or invalid")
	}
	if command == "" || len(command) > MaxBody || strings.ContainsAny(command, "\x00\r\n") || !utf8.ValidString(command) {
		return "", errors.New("RCON command must be a nonempty single UTF-8 line of at most 4086 bytes")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return "", ErrTransport
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if conn.SetDeadline(deadline) != nil {
		return "", ErrTransport
	}
	if err = write(conn, packet{1, 3, password}); err != nil {
		return "", err
	}
	auth, err := read(conn)
	if err != nil {
		return "", err
	}
	if auth.kind == 0 && auth.id == 1 && auth.body == "" {
		auth, err = read(conn)
		if err != nil {
			return "", err
		}
	}
	if auth.kind != 2 || auth.body != "" {
		return "", ErrProtocol
	}
	if auth.id == -1 {
		return "", ErrAuthentication
	}
	if auth.id != 1 {
		return "", ErrProtocol
	}
	if err = ctx.Err(); err != nil {
		return "", ErrTransport
	}
	if err = write(conn, packet{2, 2, command}); err != nil {
		return "", err
	}
	// SRCDS mirrors an empty RESPONSE_VALUE after all command response chunks.
	// Engines without this completion behavior are explicitly unsupported.
	if err = write(conn, packet{3, 0, ""}); err != nil {
		return "", err
	}
	var response strings.Builder
	for i := 0; i < 128; i++ {
		p, err := read(conn)
		if err != nil {
			return "", err
		}
		if p.kind != 0 {
			return "", ErrProtocol
		}
		if p.id == 3 && p.body == "" {
			value := response.String()
			if !utf8.ValidString(value) || strings.ContainsRune(value, 0) || strings.Count(value, "\n") > 255 {
				return "", ErrProtocol
			}
			return strings.ReplaceAll(value, password, "[REDACTED]"), nil
		}
		if p.id != 2 || response.Len()+len(p.body) > MaxResponse {
			return "", ErrProtocol
		}
		response.WriteString(p.body)
	}
	return "", ErrProtocol
}
