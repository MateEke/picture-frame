package wifi

import (
	"context"
	"os/exec"
	"strings"
)

type Commander interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
}

type execCommander struct{}

func (execCommander) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput() //nolint:gosec
}

// parseTerseFields splits a single nmcli -t output line on unescaped ':' and
// un-escapes '\:' → ':' and '\\' → '\' in each field.
// nmcli escapes the field separator inside values (e.g. BSSIDs: "4A\:A9\:8A:...").
func parseTerseFields(line string) []string {
	line = strings.TrimRight(line, "\r")
	var fields []string
	var cur strings.Builder
	i := 0
	for i < len(line) {
		if line[i] == '\\' && i+1 < len(line) {
			switch line[i+1] {
			case ':':
				cur.WriteByte(':')
				i += 2
				continue
			case '\\':
				cur.WriteByte('\\')
				i += 2
				continue
			}
		}
		if line[i] == ':' {
			fields = append(fields, cur.String())
			cur.Reset()
			i++
			continue
		}
		cur.WriteByte(line[i])
		i++
	}
	fields = append(fields, cur.String())
	return fields
}
