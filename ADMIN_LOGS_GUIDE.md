# Picture Frame: Admin Page & Logs Guide

## Accessing the Admin Page

The picture frame runs a Go binary that serves both:
- A kiosk display for showing photos
- A browser-based admin interface for configuration

### Production Mode (default)
In production, the admin UI is embedded in the binary and served at:
```
http://<PI_IP_ADDRESS>/admin
```
or via mDNS hostname:
```
http://pictureframe-XXXX.local/admin
```

### Development Mode
In development (when running `go run ./cmd/picture-frame`), the server proxies to a Vite dev server:
```
http://localhost:5173/admin
```
Make sure to start the dev server first: `cd web && npm run dev`

### Admin API Endpoints
All admin API endpoints are under `/api/` prefix and require authentication:

| Endpoint | Method | Description |
|---|---|---|
| `/api/screen` | GET | Get screen state (on/off, auto mode) |
| `/api/screen` | POST | Set screen state (on/off) |
| `/api/library` | GET | Get library info and sync status |
| `/api/library/sync` | POST | Trigger remote library sync |
| `/api/system/info` | GET | Version, uptime, hostname, IP |
| `/api/system/update` | GET | Self-update status |
| `/api/system/update` | POST | Check for updates |
| `/api/system/update/check` | POST | Check for newer version now |
| `/api/heartbeat` | POST | Record kiosk heartbeat |
| `/api/immich/albums` | POST | List Immich albums |
| `/api/wifi` | — | WiFi management (captive portal) |

### Authentication
The admin UI and API use built-in authentication. The default password is set during initial setup. If you need to reset it, use the `--hash-password` flag or check the config.

## Viewing Application Logs

The application uses Go's `slog` package and outputs structured text logs to **stdout**.

### When running as a binary
```bash
# See real-time logs
./picture-frame

# Or with reduced output
./picture-frame --log-level=warn
```

Available log levels (in order of verbosity):
- `debug` — Detailed debugging information
- `info` — General operational information (default)
- `warn` — Warnings about non-critical issues
- `error` — Error events

### Using systemd (recommended for Raspberry Pi)
If installed via the install script or systemd service:

```bash
# View recent journal entries
journalctl -u picture-frame.service -f

# View last 50 lines
journalctl -u picture-frame.service -n 50

# Filter by priority
journalctl -u picture-frame.service -p error
```

### Docker / Container
```bash
docker logs -f picture-frame
```

### Configuring Log Level
Edit your `config.toml` or `runtime-overrides.toml`:

```toml
# Set log level in config
log_level = "debug"
```

Or override at runtime:
```bash
./picture-frame --overrides=runtime-overrides.toml
```

## Common Diagnostic Commands

### 1. Check if the frame is running
```bash
# Check process
ps aux | grep picture-frame

# Check listening port
ss -tlnp | grep :80
# or
ss -tlnp | grep :8080
```

### 2. Check network connectivity
```bash
# Ping the device
ping pictureframe-XXXX.local

# Check if HTTP is responding
curl http://pictureframe-XXXX.local/healthz
curl http://pictureframe-XXXX.local/api/system/info
```

### 3. Restart the frame
```bash
# Using systemd
systemctl restart picture-frame.service

# Or re-execute the binary (POST /api/system/restart)
curl -X POST http://pictureframe-XXXX.local/api/system/restart
```

### 4. View config state
```bash
# Get system info including config details
curl http://pictureframe-XXXX.local/api/system/info

# Get library status
curl http://pictureframe-XXXX.local/api/library
```

## Troubleshooting Surface Go/3 Specifics

When running on a Surface device:

1. **Display rotation**: The frame supports rotation via the admin UI. Check `/api/system/info` for rotation status.

2. **WiFi management**: The admin page has WiFi configuration. If the Surface is in a new network, use the captive portal that appears automatically.

3. **Screen blanking**: Motion-aware power management is controlled from the admin UI.

4. **Logs for Surface-specific issues**: Look for these in the logs:
   - `wlr-randr` — display rotation and transform
   - `policy` — screen power management decisions
   - `mqtt` — Bluetooth/MQTT sensor connectivity

## Quick Start Commands

```bash
# 1. Start the frame and see logs
./picture-frame 2>&1 | head -50

# 2. In another terminal, check the admin API
curl http://localhost:80/api/system/info

# 3. Access the admin UI in a browser
#    http://<surface-ip>/admin

# 4. To change log level at runtime:
./picture-frame --log-level=debug
```