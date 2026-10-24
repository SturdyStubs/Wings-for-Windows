package main

import "testing"

func TestTrustedPanelNodeURLs(t *testing.T) {
	for _, tc := range []struct {
		address, scheme, path string
		signed, want          bool
	}{
		{"https://windows.example/download/file?token=test", "https", "/download/file", true, true},
		{"https://windows.example:443/download/file?token=test", "https", "/download/file", true, true},
		{"https://WINDOWS.example:443/download/file?token=test", "https", "/download/file", true, true},
		{"https://other.example/download/file?token=test", "https", "/download/file", true, false},
		{"http://windows.example/download/file?token=test", "https", "/download/file", true, false},
		{"https://windows.example:8443/download/file?token=test", "https", "/download/file", true, false},
		{"https://user:secret@windows.example/download/file?token=test", "https", "/download/file", true, false},
		{"https://windows.example/download/file?token=test#fragment", "https", "/download/file", true, false},
		{"https://windows.example/download/file?token=test&token=two", "https", "/download/file", true, false},
		{"https://windows.example/download/file?token=test&extra=true", "https", "/download/file", true, false},
		{"https://windows.example/download/file?token=", "https", "/download/file", true, false},
		{"https://windows.example/other?token=test", "https", "/download/file", true, false},
		{"wss://windows.example/socket", "wss", "/socket", false, true},
		{"wss://windows.example:443/socket", "wss", "/socket", false, true},
		{"wss://windows.example/socket?token=test", "wss", "/socket", false, false},
		{"ws://windows.example/socket", "wss", "/socket", false, false},
	} {
		t.Run(tc.address, func(t *testing.T) {
			if got := trustedNodeURL(tc.address, "https://windows.example", tc.scheme, tc.path, tc.signed); got != tc.want {
				t.Fatal(got, tc.want)
			}
		})
	}
}
