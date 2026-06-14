package httpapi

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/MateEke/picture-frame/internal/version"
)

func TestSystemInfoReportsVersionAndUptime(t *testing.T) {
	orig := processStart
	t.Cleanup(func() { processStart = orig })
	processStart = time.Now().Add(-90 * time.Second)
	info := systemInfo()

	if info.Version != version.Version {
		t.Errorf("version: got %q, want %q", info.Version, version.Version)
	}
	if info.Platform != version.Platform {
		t.Errorf("platform: got %q, want %q", info.Platform, version.Platform)
	}
	if _, err := time.ParseDuration(info.Uptime); err != nil {
		t.Errorf("uptime %q is not a valid duration: %v", info.Uptime, err)
	}
}

func TestSystemInfoHostnameError(t *testing.T) {
	orig := osHostname
	t.Cleanup(func() { osHostname = orig })
	osHostname = func() (string, error) { return "", errors.New("boom") }

	if got := systemInfo().Hostname; got != "" {
		t.Errorf("hostname: got %q, want empty on error", got)
	}
}

func TestSystemInfoInterfaceAddrsError(t *testing.T) {
	orig := interfaceAddrs
	t.Cleanup(func() { interfaceAddrs = orig })
	interfaceAddrs = func() ([]net.Addr, error) { return nil, errors.New("boom") }

	if got := systemInfo().IP; got != "" {
		t.Errorf("ip: got %q, want empty on error", got)
	}
}

func TestSystemInfoPicksFirstIPv4(t *testing.T) {
	orig := interfaceAddrs
	t.Cleanup(func() { interfaceAddrs = orig })
	interfaceAddrs = func() ([]net.Addr, error) {
		return []net.Addr{
			&net.IPNet{IP: net.IPv6loopback},            // loopback: skipped
			&net.IPNet{IP: net.ParseIP("fe80::1")},      // non-IPv4: skipped
			&net.IPNet{IP: net.ParseIP("192.168.1.42")}, // chosen
			&net.IPNet{IP: net.ParseIP("192.168.1.99")}, // never reached
		}, nil
	}

	if got := systemInfo().IP; got != "192.168.1.42" {
		t.Errorf("ip: got %q, want 192.168.1.42", got)
	}
}

func TestPrimaryIPv4(t *testing.T) {
	cases := []struct {
		name  string
		addrs []net.Addr
		want  string
	}{
		{"empty", nil, ""},
		{"non-ipnet", []net.Addr{&net.IPAddr{IP: net.ParseIP("10.0.0.1")}}, ""},
		{"loopback only", []net.Addr{&net.IPNet{IP: net.ParseIP("127.0.0.1")}}, ""},
		{"ipv6 only", []net.Addr{&net.IPNet{IP: net.ParseIP("2001:db8::1")}}, ""},
		{"ipv4", []net.Addr{&net.IPNet{IP: net.ParseIP("10.1.2.3")}}, "10.1.2.3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := primaryIPv4(tc.addrs); got != tc.want {
				t.Errorf("primaryIPv4: got %q, want %q", got, tc.want)
			}
		})
	}
}
