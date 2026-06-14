# Deployment internals (Raspberry Pi)

End-user installation, requirements, options, and first setup live in the documentation:
<https://picture-frame-2kf.pages.dev/getting-started/install/>. The installer (`install.sh`) handles all
of it, and `install.sh --help` lists every flag.

This file covers the system-level details behind that install: how the kiosk stack is wired, and the
gotchas worth knowing when maintaining or debugging a deployment.

## Display backends

- **`wlopm` (default, recommended).** Full KMS (`dtoverlay=vc4-kms-v3d`) via the labwc compositor,
  with screen power through `wlr-output-power-management-v1` (real DPMS that keeps the output
  configured). The installer also pins the HDMI connector in `cmdline.txt` so a DPMS-off panel does
  not drop the output. Units: `labwc.service`, `kiosk-browser.service` (cog as a labwc client), and
  `kiosk-backend.service`. labwc and the browser are separate services (not `cage -- cog`), so a cog
  crash restarts only cog while labwc holds the panel power state, and a manually-off screen does not
  flash back on.
- **`vcgencmd` (legacy fallback, `--display-backend vcgencmd`).** cog on DRM/GBM with no compositor
  (`dtoverlay=vc4-fkms-v3d` plus `gpu_mem=128`), screen power via `vcgencmd`. Lighter on RAM and CPU,
  but it showed occasional full-Pi instability in testing that the wlopm path does not, so prefer
  wlopm. Units: `kiosk-browser-drm.service` and `kiosk-backend.service`.

Do **not** use `wlr-randr --off` for screen power: it disables the output, tearing it out of the
layout (a modeset flash plus view-placement errors).

## Notes and gotchas

- **Shared socket.** Both the browser and backend point at `XDG_RUNTIME_DIR=/run/picture-frame` and
  `WAYLAND_DISPLAY=wayland-0`, the socket labwc creates. If `ls /run/picture-frame` shows a different
  name, update `WAYLAND_DISPLAY` in `kiosk-browser.service` and `kiosk-backend.service`.
- **0700.** `RuntimeDirectoryMode=0700` is required. libwayland rejects a 0755 `XDG_RUNTIME_DIR`
  ("Unable to open Wayland socket: Invalid argument").
- **Seat.** labwc and cog run as `User=` system services with no login session, so libseat needs
  **seatd** (installed and enabled by install.sh, ordered before the compositor). Without it both the
  logind and builtin backends fail ("Could not get primary session" / "Could not open tty0"). No
  group change is needed on Debian.
- **Bluetooth / BLE sensors.** Fresh Pi OS soft-blocks Bluetooth via rfkill, so a configured BLE
  sensor fails with "adaptor is not powered". install.sh runs `rfkill unblock bluetooth` (persisted
  by systemd-rfkill), and BlueZ then auto-powers the adapter. The adapter is selected by
  `bluetooth_adapter` (`hciN`) in config, and a USB dongle's index depends on enumeration order.
- **Automatic OS security updates.** On by default (a prompt, or `--no-unattended-upgrades` to skip).
  install.sh installs `unattended-upgrades` and turns on the periodic run via debconf, and it adds no
  origins of its own, so Debian's security-only default stands. There is **no** auto-reboot policy:
  userspace fixes apply live, while kernel fixes wait for the next power cycle rather than forcing a
  Zero through a multi-minute, screen-flashing reboot. This is separate from the app's own
  self-updater (`[updater]` in `config.toml`).
- **Persisted intent.** The backend stores the manual on/off state in
  `@@INSTALL_DIR@@/screen-state` (flag `-screen-state`) so a manually-off screen stays off across a
  backend restart.
- **Uploads / PrivateTmp.** `ProtectSystem=strict` makes the host `/tmp` read-only,
  so upload multipart spills fail ("read-only file system"). `PrivateTmp=true` gives
  the backend its own writable `/tmp` (a private tmpfs, wiped on restart).
