package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/MateEke/picture-frame/internal/wifi"
)

type WiFiManager interface {
	Status() wifi.WiFiState
	Scan(ctx context.Context) ([]wifi.WiFiNetwork, error)
	Connect(ctx context.Context, ssid, pass string, hidden bool) error
	Forget(ctx context.Context, ssid string) error
	Configure(ctx context.Context, enabled bool, ssid string, pw *string) error
}

type GetWiFiStatusOutput struct {
	Body wifi.WiFiState
}

type GetWiFiNetworksOutput struct {
	Body []wifi.WiFiNetwork
}

type WiFiConnectRequest struct {
	SSID     string `json:"ssid" minLength:"1" doc:"Network SSID to connect to"`
	Password string `json:"password,omitempty" doc:"Network password"`
	Hidden   bool   `json:"hidden,omitempty" doc:"Whether the network is hidden (non-broadcasting)"`
}

type WiFiConnectInput struct {
	Body WiFiConnectRequest
}

type WiFiForgetInput struct {
	SSID string `path:"ssid" doc:"Network SSID to forget"`
}

type APRequest struct {
	Enabled bool   `json:"enabled" doc:"Whether to enable the access point"`
	SSID    string `json:"ssid" doc:"Access point SSID"`
	// Password is a pointer so an omitted field (keep the stored key) is
	// distinguishable from an explicit "" (clear it / open AP).
	Password *string `json:"password,omitempty" doc:"Access point password (omit to keep current, empty string to clear)"`
}

type ConfigureAPInput struct {
	Body APRequest
}

type ConfigureAPOutput struct {
	Body wifi.WiFiState
}

func (s *server) registerWiFiRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-wifi-status",
		Method:      http.MethodGet,
		Path:        "/api/wifi/status",
		Summary:     "Get WiFi status",
	}, func(_ context.Context, _ *struct{}) (*GetWiFiStatusOutput, error) {
		if s.wifiMgr == nil {
			return nil, huma.Error503ServiceUnavailable("wifi not available")
		}
		return &GetWiFiStatusOutput{Body: s.wifiMgr.Status()}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-wifi-networks",
		Method:      http.MethodGet,
		Path:        "/api/wifi/networks",
		Summary:     "Scan for WiFi networks",
	}, func(ctx context.Context, _ *struct{}) (*GetWiFiNetworksOutput, error) {
		if s.wifiMgr == nil {
			return nil, huma.Error503ServiceUnavailable("wifi not available")
		}
		nets, err := s.wifiMgr.Scan(ctx)
		if err != nil {
			s.log.Error("wifi scan failed", "err", err)
			return nil, huma.Error500InternalServerError("scan failed")
		}
		return &GetWiFiNetworksOutput{Body: nets}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "wifi-connect",
		Method:        http.MethodPost,
		Path:          "/api/wifi/connect",
		Summary:       "Connect to a WiFi network",
		DefaultStatus: http.StatusAccepted,
		MaxBodyBytes:  4096,
	}, func(ctx context.Context, input *WiFiConnectInput) (*struct{}, error) {
		if s.wifiMgr == nil {
			return nil, huma.Error503ServiceUnavailable("wifi not available")
		}
		err := s.wifiMgr.Connect(ctx, input.Body.SSID, input.Body.Password, input.Body.Hidden)
		if errors.Is(err, wifi.ErrBusy) {
			return nil, huma.Error503ServiceUnavailable("wifi manager busy")
		}
		if err != nil {
			return nil, huma.Error500InternalServerError("connect failed")
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "wifi-forget",
		Method:        http.MethodDelete,
		Path:          "/api/wifi/network/{ssid}",
		Summary:       "Forget a saved WiFi network",
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, input *WiFiForgetInput) (*struct{}, error) {
		if s.wifiMgr == nil {
			return nil, huma.Error503ServiceUnavailable("wifi not available")
		}
		if err := s.wifiMgr.Forget(ctx, input.SSID); err != nil {
			s.log.Error("wifi forget failed", "ssid", input.SSID, "err", err)
			return nil, huma.Error500InternalServerError("forget failed")
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:  "configure-ap",
		Method:       http.MethodPut,
		Path:         "/api/wifi/ap",
		Summary:      "Configure the access point",
		MaxBodyBytes: 4096,
	}, func(ctx context.Context, input *ConfigureAPInput) (*ConfigureAPOutput, error) {
		if s.wifiMgr == nil {
			return nil, huma.Error503ServiceUnavailable("wifi not available")
		}
		if err := s.wifiMgr.Configure(ctx, input.Body.Enabled, input.Body.SSID, input.Body.Password); err != nil {
			s.log.Error("wifi configure failed", "err", err)
			return nil, huma.Error500InternalServerError("configure failed")
		}
		return &ConfigureAPOutput{Body: s.wifiMgr.Status()}, nil
	})
}
