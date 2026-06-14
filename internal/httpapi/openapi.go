package httpapi

//go:generate go run ../../cmd/openapi

import (
	"encoding/json"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"

	"github.com/MateEke/picture-frame/internal/config"
)

func OpenAPISpec() ([]byte, error) {
	r := chi.NewRouter()
	humaConfig := huma.DefaultConfig("Picture Frame", "1.0.0")
	humaConfig.Info.Description = "Picture frame kiosk API"
	api := humachi.New(r, humaConfig)

	s := &server{backend: config.BackendFS}
	s.registerRoutes(api)

	return json.MarshalIndent(api.OpenAPI(), "", "  ")
}
