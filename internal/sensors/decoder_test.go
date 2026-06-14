package sensors_test

import (
	"strings"
	"testing"

	"github.com/MateEke/picture-frame/internal/sensors"
)

func TestDecoders(t *testing.T) {
	cases := []struct {
		name    string
		input   []byte
		want    float64
		wantErr bool
	}{
		// int16le_div100
		{name: "int16le_div100/positive", input: []byte{0x29, 0x09}, want: 23.45},
		{name: "int16le_div100/negative", input: []byte{0xCE, 0xFE}, want: -3.06}, // -306 / 100
		{name: "int16le_div100/short", input: []byte{0x01}, wantErr: true},

		// uint16be_div10
		{name: "uint16be_div10/normal", input: []byte{0x01, 0xE5}, want: 48.5},
		{name: "uint16be_div10/zero", input: []byte{0x00, 0x00}, want: 0},
		{name: "uint16be_div10/short", input: []byte{0x01}, wantErr: true},

		// bool_nonzero
		{name: "bool_nonzero/motion", input: []byte{0x01}, want: 1},
		{name: "bool_nonzero/no_motion", input: []byte{0x00}, want: 0},
		{name: "bool_nonzero/multi_byte_any", input: []byte{0x00, 0x01}, want: 1},
		{name: "bool_nonzero/empty", input: []byte{}, wantErr: true},

		// raw_float
		{name: "raw_float/normal", input: []byte("23.4"), want: 23.4},
		{name: "raw_float/int_string", input: []byte("42"), want: 42},
		{name: "raw_float/whitespace", input: []byte("  1.5\n"), want: 1.5},
		{name: "raw_float/invalid", input: []byte("nope"), wantErr: true},

		// raw_int
		{name: "raw_int/normal", input: []byte("42"), want: 42},
		{name: "raw_int/negative", input: []byte("-7"), want: -7},
		{name: "raw_int/invalid", input: []byte("3.14"), wantErr: true},

		// onoff_to_bool
		{name: "onoff_to_bool/on", input: []byte("ON"), want: 1},
		{name: "onoff_to_bool/on_lower", input: []byte("on"), want: 1},
		{name: "onoff_to_bool/true", input: []byte("true"), want: 1},
		{name: "onoff_to_bool/True", input: []byte("True"), want: 1},
		{name: "onoff_to_bool/one", input: []byte("1"), want: 1},
		{name: "onoff_to_bool/whitespace", input: []byte("  ON\n"), want: 1},
		{name: "onoff_to_bool/off", input: []byte("OFF"), want: 0},
		{name: "onoff_to_bool/false", input: []byte("false"), want: 0},
		{name: "onoff_to_bool/zero", input: []byte("0"), want: 0},
		{name: "onoff_to_bool/empty", input: []byte(""), want: 0},
		{name: "onoff_to_bool/garbage", input: []byte("nope"), want: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decoderName := tc.name[:strings.IndexByte(tc.name, '/')]
			dec, ok := sensors.LookupDecoder(decoderName)
			if !ok {
				t.Fatalf("decoder %q not found", decoderName)
			}
			got, err := dec(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil (value=%v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLookupDecoderUnknown(t *testing.T) {
	_, ok := sensors.LookupDecoder("nonexistent")
	if ok {
		t.Fatal("expected false for unknown decoder")
	}
}
