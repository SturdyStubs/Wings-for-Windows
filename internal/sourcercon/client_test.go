package sourcercon

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func listener(t *testing.T, handle func(net.Conn)) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); handle(c) }()
		}
	}()
	return l.Addr().String()
}
func authenticated(c net.Conn) bool {
	p, err := read(c)
	if err != nil || p.id != 1 || p.kind != 3 || p.body != "fixture-secret" {
		return false
	}
	return write(c, packet{1, 0, ""}) == nil && write(c, packet{1, 2, ""}) == nil
}
func TestWireFixtureAndFragmentedRead(t *testing.T) {
	// Independently encoded little-endian size=11, ID=2, command type=2, 'x'.
	fixture := []byte{11, 0, 0, 0, 2, 0, 0, 0, 2, 0, 0, 0, 'x', 0, 0}
	var b bytes.Buffer
	if err := write(&b, packet{2, 2, "x"}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b.Bytes(), fixture) {
		t.Fatal(b.Bytes())
	}
	p, err := read(oneByte{bytes.NewReader(fixture)})
	if err != nil || p.id != 2 || p.kind != 2 || p.body != "x" {
		t.Fatal(p, err)
	}
}

type oneByte struct{ io.Reader }

func (r oneByte) Read(b []byte) (int, error) {
	if len(b) > 1 {
		b = b[:1]
	}
	return r.Reader.Read(b)
}
func TestExchangeMultiPacketAndSecretRedaction(t *testing.T) {
	address := listener(t, func(c net.Conn) {
		if !authenticated(c) {
			return
		}
		p, err := read(c)
		if err != nil || p.kind != 2 || p.body != "status" {
			return
		}
		end, err := read(c)
		if err != nil || end.id != 3 || end.kind != 0 || end.body != "" {
			return
		}
		write(c, packet{2, 0, "hello fixture-"})
		write(c, packet{2, 0, "secret\nworld \xe9"})
		write(c, packet{2, 0, "\x9b\xaa"})
		write(c, packet{3, 0, ""})
	})
	result, err := Execute(context.Background(), address, "fixture-secret", "status", time.Second)
	if err != nil || result != "hello [REDACTED]\nworld 雪" {
		t.Fatal(result, err)
	}
}

func TestAuthenticationWithoutEmptyPreludeAndEmptyReply(t *testing.T) {
	address := listener(t, func(c net.Conn) {
		p, err := read(c)
		if err != nil || p.kind != 3 {
			return
		}
		write(c, packet{p.id, 2, ""})
		if _, err = read(c); err != nil {
			return
		}
		end, err := read(c)
		if err != nil {
			return
		}
		write(c, packet{end.id, 0, ""})
	})
	value, err := Execute(context.Background(), address, "fixture-secret", "status", time.Second)
	if err != nil || value != "" {
		t.Fatal(value, err)
	}
}
func TestRefusesMalformedOrUnconfirmedResponses(t *testing.T) {
	for _, mode := range []string{"auth-denied", "auth-id", "auth-body", "response-id", "response-type", "bad-padding", "negative-size", "huge-size", "truncated", "timeout", "no-completion", "packet-limit", "output-limit", "invalid-text", "line-limit"} {
		t.Run(mode, func(t *testing.T) {
			var commands atomic.Int32
			address := listener(t, func(c net.Conn) {
				if mode == "auth-denied" {
					read(c)
					write(c, packet{-1, 2, ""})
					return
				}
				if mode == "auth-id" {
					read(c)
					write(c, packet{99, 2, ""})
					return
				}
				if mode == "auth-body" {
					read(c)
					write(c, packet{1, 2, "fixture-secret"})
					return
				}
				if !authenticated(c) {
					return
				}
				if _, err := read(c); err != nil {
					return
				}
				commands.Add(1)
				read(c)
				switch mode {
				case "response-id":
					write(c, packet{99, 0, "secret"})
				case "response-type":
					write(c, packet{2, 2, "secret"})
				case "bad-padding":
					c.Write([]byte{10, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 1, 0})
				case "negative-size":
					binary.Write(c, binary.LittleEndian, int32(-1))
				case "huge-size":
					binary.Write(c, binary.LittleEndian, int32(1<<30))
				case "truncated":
					c.Write([]byte{10, 0, 0, 0, 2})
				case "timeout":
					time.Sleep(150 * time.Millisecond)
				case "no-completion":
					write(c, packet{2, 0, "not complete"})
				case "packet-limit":
					for i := 0; i < 129; i++ {
						if write(c, packet{2, 0, ""}) != nil {
							return
						}
					}
				case "output-limit":
					for i := 0; i < 18; i++ {
						if write(c, packet{2, 0, strings.Repeat("x", MaxBody)}) != nil {
							return
						}
					}
				case "invalid-text":
					write(c, packet{2, 0, "\xff"})
					write(c, packet{3, 0, ""})
				case "line-limit":
					write(c, packet{2, 0, strings.Repeat("\n", 256)})
					write(c, packet{3, 0, ""})
				}
			})
			value, err := Execute(context.Background(), address, "fixture-secret", "status", 100*time.Millisecond)
			if err == nil || value != "" || strings.Contains(err.Error(), "fixture-secret") {
				t.Fatal(value, err)
			}
			if commands.Load() > 1 {
				t.Fatal("command retried")
			}
		})
	}
}
func TestCancellationAndInputBounds(t *testing.T) {
	started := make(chan struct{})
	address := listener(t, func(c net.Conn) {
		if authenticated(c) {
			read(c)
			close(started)
			io.Copy(io.Discard, c)
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := Execute(ctx, address, "fixture-secret", "status", 5*time.Second); done <- err }()
	<-started
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled exchange succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation blocked")
	}
	for _, value := range []struct{ address, password, command string }{{"example.com:1", "p", "x"}, {"0.0.0.0:1", "p", "x"}, {address, "", "x"}, {address, "p\x00", "x"}, {address, "p", "x\ny"}, {address, "p", strings.Repeat("x", MaxBody+1)}} {
		if _, err := Execute(context.Background(), value.address, value.password, value.command, time.Second); err == nil {
			t.Fatal("invalid request accepted")
		}
	}
}
