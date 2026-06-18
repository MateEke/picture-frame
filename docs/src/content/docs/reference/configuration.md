---
title: Configuration file
description: Every setting in config.toml, with its type, default, and meaning.
---

This page lists every setting in `config.toml`. For where the file lives, how `config.toml` and
`runtime-overrides.toml` layer, and how to apply a change, see
[Configuration basics](/getting-started/configuration/). Most of these settings also have a
control in the admin interface, covered in the [User Manual](/manual/dashboard/).

A row marked **(live)** is applied at once when you change it in the **admin interface**, which
saves it to `runtime-overrides.toml` and updates the running frame in place. Every other change
needs a restart. Editing `config.toml` by hand is a different matter: the file is read only at
startup, so any hand-edit needs a restart, even for a (live) field. Durations are written as Go
duration strings, such as `500ms`, `120s`, `10m`, or `2h`.

## Top level

| Key                 | Type   | Default | Description                                                                                    |
| ------------------- | ------ | ------- | ---------------------------------------------------------------------------------------------- |
| `addr`              | string | `:80`   | HTTP listen address. Port 80 keeps the hostname-only URL and the captive-portal probe working. |
| `log_level`         | string | `info`  | Logging verbosity: `debug`, `info`, `warn`, or `error`. **(live)**                             |
| `bluetooth_adapter` | string | `hci0`  | HCI device for Bluetooth [sensors](/manual/sensors/).                                          |

## `[display]`

See [Slideshow & display](/manual/slideshow-display/).

| Key           | Type     | Default | Description                                                                                            |
| ------------- | -------- | ------- | ------------------------------------------------------------------------------------------------------ |
| `blank_after` | duration | `20m`   | Idle time with no motion before the screen blanks. `0s` disables it. Needs a motion sensor. **(live)** |
| `backend`     | string   | `wlopm` | Screen-power backend, `wlopm` or `vcgencmd`. Switching it means re-running the installer.              |
| `output`      | string   | (auto)  | Wayland connector for the wlopm backend, such as `HDMI-A-1`.                                           |
| `locale`      | string   | `en-US` | BCP-47 locale for the clock and date on the frame. **(live)**                                          |

### `[display.labels]`

Captions under the readings on the frame. An empty value hides that caption. **(live)**

| Key        | Type   | Default | Description                       |
| ---------- | ------ | ------- | --------------------------------- |
| `outside`  | string | (empty) | Caption for the outside reading.  |
| `inside`   | string | (empty) | Caption for the inside reading.   |
| `humidity` | string | (empty) | Caption for the humidity reading. |

## `[slideshow]`

| Key              | Type     | Default  | Description                                                                                           |
| ---------------- | -------- | -------- | ----------------------------------------------------------------------------------------------------- |
| `interval`       | duration | `120s`   | How long each photo shows before advancing. **(live)**                                                |
| `randomize`      | boolean  | `false`  | Shuffle the order on each full cycle. **(live)**                                                      |
| `split_screen`   | boolean  | `true`   | Pair mismatched-orientation photos side by side instead of cropping. **(live)**                       |
| `pair_threshold` | float    | `1.5`    | How far a photo's aspect must differ from the screen's to pair, as a factor (must be > 1). **(live)** |
| `images_dir`     | string   | `images` | Root folder for image storage.                                                                        |

## `[library]`

See [Photos](/manual/photos/).

| Key       | Type   | Default | Description                                                        |
| --------- | ------ | ------- | ------------------------------------------------------------------ |
| `backend` | string | `fs`    | `fs` for local uploads, or `immich` to sync a shared Immich album. |

### `[library.immich]`

Used only when `backend = "immich"`.

| Key              | Type     | Default | Description                                                              |
| ---------------- | -------- | ------- | ------------------------------------------------------------------------ |
| `share_url`      | string   | (empty) | Immich shared-album link. Required for the immich backend.               |
| `share_password` | string   | (empty) | Share password, exchanged for a session token on Immich 2.6.0 and newer. |
| `sync_interval`  | duration | `15m`   | How often to reconcile with the album.                                   |

## `[[sensor]]`

One block per sensor. See [Sensors](/manual/sensors/). Every sensor needs a unique `id`, and each
`role` plus `kind` pair must be unique across all sensors.

| Key    | Type   | Default  | Description                                              |
| ------ | ------ | -------- | -------------------------------------------------------- |
| `id`   | string | required | Unique name for the sensor.                              |
| `type` | string | required | `ble`, `mqtt-subscriber`, or `mock`.                     |
| `role` | string | (id)     | Groups readings by place, such as `inside` or `outside`. |

**Reading kinds** are `temperature`, `humidity`, and `motion`.

### Bluetooth (`type = "ble"`)

| Key             | Type     | Default  | Description                                                                        |
| --------------- | -------- | -------- | ---------------------------------------------------------------------------------- |
| `mac`           | string   | required | Device MAC address.                                                                |
| `address_type`  | string   | (empty)  | `public` or `random`.                                                              |
| `poll_interval` | duration | `80s`    | Fallback read interval between notifications.                                      |
| `reset_after`   | duration | disabled | Power-cycle the Bluetooth adapter after connecting fails this long. `0s` disables. |

Each `[[sensor.characteristic]]` maps one GATT characteristic:

| Key       | Type   | Default  | Description                                |
| --------- | ------ | -------- | ------------------------------------------ |
| `uuid`    | string | required | GATT characteristic UUID.                  |
| `kind`    | string | required | Reading kind this characteristic provides. |
| `decoder` | string | required | Decoder for the raw bytes (see below).     |

### MQTT (`type = "mqtt-subscriber"`)

Requires `[mqtt].broker`.

| Key          | Type   | Default  | Description                                                         |
| ------------ | ------ | -------- | ------------------------------------------------------------------- |
| `topic`      | string | required | MQTT topic to read.                                                 |
| `kind`       | string | required | Reading kind this topic provides.                                   |
| `parser`     | string | required | Payload decoder (see below).                                        |
| `json_field` | string | (empty)  | Dotted path into a JSON payload, such as `main.temp`. Objects only. |

### Mock (`type = "mock"`)

| Key             | Type     | Default | Description                  |
| --------------- | -------- | ------- | ---------------------------- |
| `poll_interval` | duration | (none)  | How often to emit a reading. |

Each `[[sensor.mock_reading]]`:

| Key     | Type   | Default  | Description                                                    |
| ------- | ------ | -------- | -------------------------------------------------------------- |
| `kind`  | string | required | Reading kind.                                                  |
| `value` | float  | `0`      | Starting value.                                                |
| `delta` | float  | `0`      | Added to `value` after each full cycle. `0` holds it constant. |

### Decoders and parsers

A Bluetooth `decoder` and an MQTT `parser` draw from the same set:

| Name             | Use                                                           |
| ---------------- | ------------------------------------------------------------- |
| `int16le_div100` | Little-endian signed 16-bit integer ÷ 100 (`2345` → `23.45`). |
| `uint16be_div10` | Big-endian unsigned 16-bit integer ÷ 10 (`485` → `48.5`).     |
| `bool_nonzero`   | Any non-zero byte becomes 1.                                  |
| `raw_float`      | Text holding a number, such as `23.4`.                        |
| `raw_int`        | Text holding an integer, such as `42`.                        |
| `onoff_to_bool`  | `ON`, `on`, `true`, or `1` become 1, anything else 0.         |

## `[weather]`

See [Weather](/manual/weather/).

| Key              | Type     | Default  | Description                                                                     |
| ---------------- | -------- | -------- | ------------------------------------------------------------------------------- |
| `api_key`        | string   | (empty)  | OpenWeatherMap API key. Empty disables weather.                                 |
| `lat`            | float    | `0.0`    | Latitude in decimal degrees.                                                    |
| `lon`            | float    | `0.0`    | Longitude in decimal degrees.                                                   |
| `poll_interval`  | duration | `10m`    | How often to fetch conditions. **(live)**                                       |
| `retry_interval` | duration | `30s`    | First delay after a failed fetch, backing off up to `poll_interval`. **(live)** |
| `units`          | string   | `metric` | `standard` (K), `metric` (°C), or `imperial` (°F).                              |

## `[mqtt]`

Connection shared by the bridge and MQTT sensors. See [Home Assistant](/manual/home-assistant/).
A connection opens only when the bridge is enabled or an MQTT sensor exists.

| Key         | Type   | Default         | Description                                  |
| ----------- | ------ | --------------- | -------------------------------------------- |
| `broker`    | string | (empty)         | `tcp://` or `ssl://` host and port.          |
| `username`  | string | (empty)         | Broker username, if required.                |
| `password`  | string | (empty)         | Broker password, if required.                |
| `client_id` | string | `picture-frame` | Client identifier. Keep it unique per frame. |

### `[mqtt.bridge]`

Outbound Home Assistant auto-discovery.

| Key                | Type     | Default         | Description                                                             |
| ------------------ | -------- | --------------- | ----------------------------------------------------------------------- |
| `enabled`          | boolean  | `false`         | Publish sensors and the screen switch to Home Assistant.                |
| `node_id`          | string   | `picture_frame` | Device id and unique-id prefix, distinct per frame.                     |
| `base_topic`       | string   | `picture-frame` | Namespace for state, command, and availability topics.                  |
| `discovery_prefix` | string   | `homeassistant` | Must match Home Assistant's MQTT discovery prefix.                      |
| `stale_after`      | duration | `10m`           | Mark a sensor offline if no reading arrives within this. `0s` disables. |

## `[wifi]`

See [Wi-Fi](/manual/wifi/).

| Key                     | Type    | Default        | Description                                                        |
| ----------------------- | ------- | -------------- | ------------------------------------------------------------------ |
| `ap_ssid`               | string  | `PictureFrame` | Recovery hotspot name. Clear to disable the fallback.              |
| `ap_password`           | string  | (empty)        | Hotspot WPA2 passphrase. Empty means an open hotspot.              |
| `ap_timeout_minutes`    | integer | `3`            | Minutes off-network before the hotspot is raised.                  |
| `scan_interval_minutes` | integer | `5`            | Minutes between scans for a known network while the hotspot is up. |

## `[auth]`

See [Security](/manual/security/).

| Key             | Type   | Default | Description                                                                                                    |
| --------------- | ------ | ------- | -------------------------------------------------------------------------------------------------------------- |
| `password_hash` | string | (empty) | bcrypt hash of the admin password. Empty leaves the interface open. Set from the admin interface, not by hand. |

## `[updater]`

See [Software updates](/manual/updates/).

| Key            | Type    | Default | Description                                                                                     |
| -------------- | ------- | ------- | ----------------------------------------------------------------------------------------------- |
| `auto_update`  | boolean | `true`  | Install same-line updates overnight.                                                            |
| `update_hour`  | integer | `2`     | Local hour (0–23) for the scheduled check and install.                                          |
| `github_repo`  | string  | (empty) | Release source as `owner/name`. Empty tracks the official releases.                             |
| `github_token` | string  | (empty) | Authenticates a private release source (the `PF_GITHUB_TOKEN` environment variable also works). |
