package sensors

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Decoder converts raw bytes from a sensor characteristic or MQTT payload
// into a float64 value.
type Decoder func([]byte) (float64, error)

var decoders = map[string]Decoder{
	"int16le_div100": decodeInt16LEDiv100,
	"uint16be_div10": decodeUint16BEDiv10,
	"bool_nonzero":   decodeBoolNonzero,
	"raw_float":      decodeRawFloat,
	"raw_int":        decodeRawInt,
	"onoff_to_bool":  decodeOnoffToBool,
}

// LookupDecoder returns the named decoder, or false if unknown.
func LookupDecoder(name string) (Decoder, bool) {
	d, ok := decoders[name]
	return d, ok
}

// DecoderNames returns sorted decoder registry keys. MQTT-subscriber sensors use
// the same registry for parsers (via LookupDecoder), so this list covers both.
func DecoderNames() []string {
	names := make([]string, 0, len(decoders))
	for k := range decoders {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// decodeInt16LEDiv100 reads a little-endian int16 / 100 (e.g. BLE temp 2345 → 23.45).
func decodeInt16LEDiv100(b []byte) (float64, error) {
	if len(b) < 2 {
		return 0, fmt.Errorf("int16le_div100: need ≥2 bytes, got %d", len(b))
	}
	var v int16
	if err := binary.Read(bytes.NewReader(b), binary.LittleEndian, &v); err != nil {
		return 0, fmt.Errorf("int16le_div100: %w", err)
	}
	return float64(v) / 100, nil
}

// decodeUint16BEDiv10 reads a big-endian uint16 / 10 (e.g. BLE humidity 485 → 48.5).
func decodeUint16BEDiv10(b []byte) (float64, error) {
	if len(b) < 2 {
		return 0, fmt.Errorf("uint16be_div10: need ≥2 bytes, got %d", len(b))
	}
	return float64(binary.BigEndian.Uint16(b)) / 10, nil
}

// decodeBoolNonzero returns 1 if any byte is non-zero (e.g. BLE motion), else 0.
func decodeBoolNonzero(b []byte) (float64, error) {
	if len(b) == 0 {
		return 0, fmt.Errorf("bool_nonzero: empty payload")
	}
	for _, v := range b {
		if v != 0 {
			return 1, nil
		}
	}
	return 0, nil
}

// decodeRawFloat parses a UTF-8 string as a float64 (e.g. MQTT "23.4").
func decodeRawFloat(b []byte) (float64, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
	if err != nil {
		return 0, fmt.Errorf("raw_float: %w", err)
	}
	return v, nil
}

// decodeRawInt parses a UTF-8 string as a base-10 integer (e.g. MQTT "42").
func decodeRawInt(b []byte) (float64, error) {
	v, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("raw_int: %w", err)
	}
	return float64(v), nil
}

// decodeOnoffToBool returns 1 for "ON"/"on"/"true"/"1", 0 otherwise.
func decodeOnoffToBool(b []byte) (float64, error) {
	switch strings.ToLower(strings.TrimSpace(string(b))) {
	case "on", "true", "1":
		return 1, nil
	default:
		return 0, nil
	}
}
