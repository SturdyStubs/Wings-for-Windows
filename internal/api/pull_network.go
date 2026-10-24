package api

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// Conservative rejection of special-use/relay/documentation networks. This is
// deliberately narrower than IsGlobalUnicast, which includes special ranges.
var blockedPullNetworks = func() []netip.Prefix {
	var out []netip.Prefix
	for _, p := range strings.Fields("0.0.0.0/8 10.0.0.0/8 100.64.0.0/10 127.0.0.0/8 169.254.0.0/16 172.16.0.0/12 192.0.0.0/24 192.0.2.0/24 192.88.99.0/24 192.168.0.0/16 198.18.0.0/15 198.51.100.0/24 203.0.113.0/24 224.0.0.0/4 240.0.0.0/4 2001::/23 2001:db8::/32 2002::/16 3fff::/20") {
		out = append(out, netip.MustParsePrefix(p))
	}
	return out
}()

func publicPullAddress(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || ip.Zone() != "" || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	if ip.Is6() && !netip.MustParsePrefix("2000::/3").Contains(ip) {
		return false
	}
	for _, prefix := range blockedPullNetworks {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}
func pullURL(raw string) (*url.URL, error) {
	if len(raw) == 0 || len(raw) > 8192 {
		return nil, errors.New("invalid remote URL")
	}
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" || u.Port() != "" && u.Port() != "443" || strings.ContainsAny(u.Hostname(), "%\\\x00\r\n ") {
		return nil, errors.New("only public HTTPS on port 443 is supported")
	}
	if ip, e := netip.ParseAddr(u.Hostname()); e == nil && !publicPullAddress(ip) {
		return nil, errors.New("remote address is restricted")
	}
	return u, nil
}

type pullLookup func(context.Context, string, string) ([]netip.Addr, error)
type pullDial func(context.Context, string, string) (net.Conn, error)

func guardedPullDial(lookup pullLookup, dial pullDial) pullDial {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(address)
		if e != nil || port != "443" {
			return nil, errors.New("remote destination is restricted")
		}
		resolveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		ips, e := lookup(resolveCtx, "ip", host)
		if e != nil || len(ips) == 0 || len(ips) > 16 {
			return nil, errors.New("remote destination unavailable")
		}
		for _, ip := range ips {
			if !publicPullAddress(ip) {
				return nil, errors.New("remote destination is restricted")
			}
		}
		// Dial the already validated numeric address, never resolve the hostname
		// again between validation and connection (including DNS rebinding).
		for i, ip := range ips {
			if i >= 4 {
				break
			}
			conn, e := dial(ctx, "tcp", net.JoinHostPort(ip.Unmap().String(), port))
			if e == nil {
				return conn, nil
			}
		}
		return nil, errors.New("remote connection failed")
	}
}
func newPullClient() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{Proxy: nil, DialContext: guardedPullDial(net.DefaultResolver.LookupNetIP, dialer.DialContext), TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, DisableCompression: true, DisableKeepAlives: true, MaxResponseHeaderBytes: 16 << 10, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 10 * time.Second}
	return &http.Client{Transport: transport, Timeout: 2 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
