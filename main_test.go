package main

import (
	"reflect"
	"testing"
)

func TestDisplayURL(t *testing.T) {
	cases := []struct {
		addr string
		want string
	}{
		{"127.0.0.1:8899", "http://127.0.0.1:8899"},
		{"0.0.0.0:9000", "http://127.0.0.1:9000"},
		{":8899", "http://127.0.0.1:8899"},
		{"[::1]:8899", "http://[::1]:8899"},
		{"localhost:8899", "http://localhost:8899"},
	}
	for _, tc := range cases {
		if got := displayURL(tc.addr); got != tc.want {
			t.Errorf("displayURL(%q) = %q, want %q", tc.addr, got, tc.want)
		}
	}
}

func TestDefaultOpenBrowser(t *testing.T) {
	cases := map[string]bool{
		"windows": true,
		"darwin":  true,
		"linux":   false,
		"freebsd": false,
	}
	for goos, want := range cases {
		if got := defaultOpenBrowser(goos); got != want {
			t.Errorf("defaultOpenBrowser(%q) = %v, want %v", goos, got, want)
		}
	}
}

func TestBrowserCommand(t *testing.T) {
	cases := []struct {
		goos string
		url  string
		cmd  string
		args []string
	}{
		{"darwin", "http://127.0.0.1:8899", "open", []string{"http://127.0.0.1:8899"}},
		{"windows", "http://127.0.0.1:8899", "rundll32", []string{"url.dll,FileProtocolHandler", "http://127.0.0.1:8899"}},
		{"linux", "http://127.0.0.1:8899", "xdg-open", []string{"http://127.0.0.1:8899"}},
	}
	for _, tc := range cases {
		cmd, args := browserCommand(tc.goos, tc.url)
		if cmd != tc.cmd || !reflect.DeepEqual(args, tc.args) {
			t.Errorf("browserCommand(%q, %q) = %q %v, want %q %v", tc.goos, tc.url, cmd, args, tc.cmd, tc.args)
		}
	}
}
