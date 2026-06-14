# web

The SvelteKit frontend for picture-frame, both the browser **admin UI** and the
on-screen **kiosk display**. It's built to static assets (adapter-static) and embedded
into the Go binary, so in production it's served by the backend, not a Node server.

## Routes

- `/admin`: dashboard, settings, photo library, and WiFi management.
- `/kiosk`: the full-screen display shown on the frame (slideshow + overlay).
- `/login`: admin sign-in when authentication is enabled.

## Development

From the repository root:

```sh
make watch   # Go backend (Air) + Vite dev server with a proxy to it
```

The dev server proxies API and SSE requests to `BACKEND_URL` (default
`http://localhost:8080`).

To work on the UI alone, without building or running the Go backend locally, point
the Vite dev server at any running frame and skip `make watch`:

```sh
BACKEND_URL=http://pictureframe-xxxx.local npm run dev
```

## API client

`src/lib/api/` is generated from the backend's OpenAPI spec. **Do not edit it by
hand.** After changing a Go handler, regenerate it from the root:

```sh
make generate
```

## Testing

```sh
npm run test:unit   # Vitest (unit + component)
npm run test:e2e    # Playwright; each test runs against its own backend instance
```

## Build

```sh
npm run build       # static assets the Go binary embeds (usually run via `make build`)
```
