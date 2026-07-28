package httpapi

import (
	"context"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/MateEke/picture-frame/internal/version"
)

var processStart = time.Now()

// Seams over OS lookups so the enumeration branches stay testable.
var (
	osHostname     = os.Hostname
	interfaceAddrs = net.InterfaceAddrs
)

// SystemInfoBody is a point-in-time snapshot for the dashboard system card.
type SystemInfoBody struct {
	Version      string   `json:"version" doc:"Build version"`
	Platform     string   `json:"platform" doc:"Build target, e.g. linux_armv6"`
	Uptime       string   `json:"uptime" doc:"Process uptime as a Go duration, e.g. 3h12m0s"`
	Hostname     string   `json:"hostname" doc:"OS hostname"`
	IP           string   `json:"ip" doc:"Primary non-loopback IPv4, empty if none"`
	CPUTempC     *float64 `json:"cpuTempC,omitempty" doc:"CPU temperature in Celsius"`
	MemUsedPct   *float64 `json:"memUsedPct,omitempty" doc:"Used memory percent"`
	SystemUptime string   `json:"systemUptime,omitempty" doc:"System uptime since boot as a Go duration"`
	Undervoltage *bool    `json:"undervoltage,omitempty" doc:"True if undervoltage detected"`
}

type getSystemInfoOutput struct {
	Body SystemInfoBody
}

func (s *server) registerSystemInfoRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-system-info",
		Method:      http.MethodGet,
		Path:        "/api/system/info",
		Summary:     "Report version, uptime, hostname and primary IP",
	}, func(ctx context.Context, _ *struct{}) (*getSystemInfoOutput, error) {
		return &getSystemInfoOutput{Body: s.systemInfo(ctx)}, nil
	})
}

func (s *server) systemInfo(ctx context.Context) SystemInfoBody {
	host, _ := osHostname()
	var ip string
	if addrs, err := interfaceAddrs(); err == nil {
		ip = primaryIPv4(addrs)
	}
	body := SystemInfoBody{
		Version:  version.Version,
		Platform: version.Platform,
		Uptime:   time.Since(processStart).Round(time.Second).String(),
		Hostname: host,
		IP:       ip,
	}
	if s.hostMetrics == nil {
		return body
	}
	m := s.hostMetrics.Read(ctx)
	if m.HasCPUTemp {
		v := m.CPUTempC
		body.CPUTempC = &v
	}
	if m.HasMem {
		v := m.MemUsedPct
		body.MemUsedPct = &v
	}
	if m.HasBootTime {
		body.SystemUptime = time.Since(m.BootTime).Round(time.Second).String()
	}
	if m.HasThrottle {
		v := m.Undervoltage
		body.Undervoltage = &v
	}
	return body
}

func primaryIPv4(addrs []net.Addr) string {
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		if v4 := ipnet.IP.To4(); v4 != nil {
			return v4.String()
		}
	}
	return ""
}
