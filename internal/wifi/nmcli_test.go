package wifi

import (
	"reflect"
	"testing"
)

func TestParseTerseFields(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			"simple three fields",
			"WPA2:MyNetwork:72",
			[]string{"WPA2", "MyNetwork", "72"},
		},
		{
			"BSSID with escaped colons",
			`4A\:A9\:8A\:E9\:DE\:BB:MySSID:60`,
			[]string{"4A:A9:8A:E9:DE:BB", "MySSID", "60"},
		},
		{
			"escaped backslash",
			`foo\\bar:baz`,
			[]string{`foo\bar`, "baz"},
		},
		{
			"open network empty security trailing field",
			"MyNet::80:",
			[]string{"MyNet", "", "80", ""},
		},
		{
			"single field no colon",
			"alone",
			[]string{"alone"},
		},
		{
			"empty string",
			"",
			[]string{""},
		},
		{
			"ssid with escaped colon",
			`Net\:work:WPA2:55`,
			[]string{"Net:work", "WPA2", "55"},
		},
		{
			"backslash at end of input",
			`foo\`,
			[]string{`foo\`},
		},
		{
			"real nmcli wifi list line format: ssid:bssid:mode:chan:rate:signal:bars:security",
			`HomeWiFi:4A\:A9\:8A\:E9\:DE\:BB:Infra:6:130 Mbit/s:72:▂▄__:WPA2`,
			[]string{"HomeWiFi", "4A:A9:8A:E9:DE:BB", "Infra", "6", "130 Mbit/s", "72", "▂▄__", "WPA2"},
		},
		{
			"WPA3-only entry",
			`GuestNet:BE\:EF\:CA\:FE\:00\:01:Infra:11:54 Mbit/s:45:▂▄__:WPA3`,
			[]string{"GuestNet", "BE:EF:CA:FE:00:01", "Infra", "11", "54 Mbit/s", "45", "▂▄__", "WPA3"},
		},
		{
			"mixed WPA2 WPA3 security field (space in value, not a separator)",
			`Mixed:AA\:BB\:CC\:DD\:EE\:FF:Infra:1:130 Mbit/s:88:▂▄▆█:WPA2 WPA3`,
			[]string{"Mixed", "AA:BB:CC:DD:EE:FF", "Infra", "1", "130 Mbit/s", "88", "▂▄▆█", "WPA2 WPA3"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseTerseFields(tc.input)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseTerseFields(%q)\n  got  %v\n  want %v", tc.input, got, tc.want)
			}
		})
	}
}
