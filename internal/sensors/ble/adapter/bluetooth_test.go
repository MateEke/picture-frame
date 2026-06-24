//go:build linux

package adapter

import "testing"

func TestParseAddress(t *testing.T) {
	const mac = "AA:BB:CC:DD:EE:FF"
	cases := []struct {
		name        string
		addressType string
		wantRandom  bool
		wantErr     bool
	}{
		{name: "random", addressType: "random", wantRandom: true},
		{name: "public", addressType: "public", wantRandom: false},
		{name: "case-insensitive", addressType: "Random", wantRandom: true},
		{name: "empty defaults to public", addressType: "", wantRandom: false},
		{name: "trims surrounding whitespace", addressType: "public ", wantRandom: false},
		{name: "unknown type errors", addressType: "publik", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addr, err := parseAddress(mac, tc.addressType)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseAddress(%q, %q) = nil error, want error", mac, tc.addressType)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseAddress(%q, %q) error: %v", mac, tc.addressType, err)
			}
			if addr.IsRandom() != tc.wantRandom {
				t.Fatalf("parseAddress(%q, %q) IsRandom = %v, want %v", mac, tc.addressType, addr.IsRandom(), tc.wantRandom)
			}
		})
	}
}

func TestParseAddressInvalidMAC(t *testing.T) {
	if _, err := parseAddress("not-a-mac", "public"); err == nil {
		t.Fatal("parseAddress with invalid MAC = nil error, want error")
	}
}

func TestDeviceObjectPath(t *testing.T) {
	cases := []struct {
		name      string
		adapterID string
		mac       string
		want      string
	}{
		{
			name:      "uppercase mac",
			adapterID: "hci0",
			mac:       "D5:CE:E6:61:5B:1D",
			want:      "/org/bluez/hci0/dev_D5_CE_E6_61_5B_1D",
		},
		{
			name:      "lowercase mac is upcased to match bluez",
			adapterID: "hci0",
			mac:       "d5:ce:e6:61:5b:1d",
			want:      "/org/bluez/hci0/dev_D5_CE_E6_61_5B_1D",
		},
		{
			name:      "non-default adapter id",
			adapterID: "hci1",
			mac:       "AA:BB:CC:DD:EE:FF",
			want:      "/org/bluez/hci1/dev_AA_BB_CC_DD_EE_FF",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(deviceObjectPath(tc.adapterID, tc.mac))
			if got != tc.want {
				t.Fatalf("deviceObjectPath(%q, %q) = %q, want %q", tc.adapterID, tc.mac, got, tc.want)
			}
		})
	}
}
