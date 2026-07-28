package adapter

import (
	"testing"
)

func TestParseThrottled(t *testing.T) {
	cases := []struct {
		input  []byte
		want   uint64
		wantOK bool
	}{
		{[]byte("throttled=0x0\n"), 0x0, true},
		{[]byte("throttled=0x50005"), 0x50005, true},
		{[]byte("throttled=0x10000\n"), 0x10000, true},
		{[]byte("unexpected\n"), 0, false},
		{[]byte("throttled=nothex\n"), 0, false},
		{[]byte(""), 0, false},
	}
	for _, tc := range cases {
		got, ok := parseThrottled(tc.input)
		if ok != tc.wantOK || got != tc.want {
			t.Errorf("input=%q: got (%#x, %v), want (%#x, %v)", tc.input, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestParseDisplayPower(t *testing.T) {
	cases := []struct {
		input   []byte
		want    string
		wantErr bool
	}{
		{[]byte("display_power=1\n"), "1", false},
		{[]byte("display_power=0\n"), "0", false},
		{[]byte("display_power=1"), "1", false},
		{[]byte("unexpected output\n"), "", true},
		{[]byte(""), "", true},
	}
	for _, tc := range cases {
		got, err := parseDisplayPower(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("input=%q: want error, got nil", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("input=%q: unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("input=%q: got %q, want %q", tc.input, got, tc.want)
		}
	}
}
