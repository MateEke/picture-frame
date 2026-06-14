package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/MateEke/picture-frame/internal/httpapi"
)

func main() {
	spec, err := httpapi.OpenAPISpec()
	if err != nil {
		fmt.Fprintf(os.Stderr, "openapi: %v\n", err)
		os.Exit(1)
	}

	const outPath = "../../web/openapi.json"
	existing, _ := os.ReadFile(outPath)
	if bytes.Equal(spec, existing) {
		return
	}

	if err := os.WriteFile(outPath, spec, 0o644); err != nil { //nolint:gosec // committed spec file, world-readable is intentional
		fmt.Fprintf(os.Stderr, "openapi: write: %v\n", err)
		os.Exit(1)
	}
}
