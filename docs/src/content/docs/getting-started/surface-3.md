---
title: Surface 3
description: Run the frame on a Microsoft Surface 3 tablet with Debian 13 and the linux-surface kernel.
---

The frame is made for a Raspberry Pi, but it runs on an x86-64 tablet too. This page covers the
Microsoft Surface 3 — the 2015 Intel Atom x7-Z8700 model, **not** the Surface Pro 3 — as a
self-contained picture frame: screen, battery, Wi-Fi and touchscreen in one enclosure.

It is a Debian 13 install with the [linux-surface](https://github.com/linux-surface/linux-surface)
kernel and the same kiosk stack the Pi uses. There is no desktop, which is deliberate: the tablet has
2 GB of RAM and a WebKit browser to run.

:::caution[Not a Raspberry Pi]
The installer detects the architecture and skips the Pi-specific steps (boot firmware overlays) on its
own, but this is a supported-by-hand target: an unreleased architecture, a patched kernel, and a
touchscreen driver with a history. Read [What works](#what-works) before you wipe anything.
:::

## What works

| Component           | Status | Notes                                                                                                                                                                       |
| ------------------- | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Display (1920×1280) | ✅     | `eDP-1`                                                                                                                                                                     |
| Screen power (DPMS) | ⚠️     | `wlopm` on Intel `eDP` is untested upstream; fall back to `blank_after = "0s"` if it fails                                                                                  |
| Touchscreen         | ⚠️     | Works, but the `surface3-spi` driver has crashed randomly since kernel 5.18. The linux-surface kernel carries the fix, enabled by default (see [Touchscreen](#touchscreen)) |
| Keyboard / touchpad | ✅     | The Type Cover works as USB HID                                                                                                                                             |
| Wi-Fi               | ⚠️     | Marvell 88W8897 (`mwifiex`) works; the driver can still die under load or after a resume. Keep suspend off                                                                  |
| Audio               | ⚠️     | Needs `firmware-sof-signed`; see [Audio](#audio) for the single-tone fix                                                                                                    |
| Battery status      | ✅     | `surface3_power`; needs reasonably recent UEFI firmware                                                                                                                     |
| Sensors (rotation)  | ✅     | `iio-sensor-proxy`                                                                                                                                                          |
| microSD reader      | ✅     | A good place to keep photos                                                                                                                                                 |
| Microphone          | ❌     | [linux-surface#681](https://github.com/linux-surface/linux-surface/issues/681)                                                                                              |
| Cameras             | ❌     | [linux-surface#209](https://github.com/linux-surface/linux-surface/issues/209)                                                                                              |

## Why Debian 13

Debian 13 (trixie) is the distribution to use here:

- the whole kiosk stack is in `main`: `cog`, `labwc`, `wlopm`, `seatd`, `wlr-randr`
- it is the same family the frame is tested on (Raspberry Pi OS **Trixie** Lite)
- Ubuntu noble has `labwc` and `wlopm` but **no `cog`**, so a desktop distribution would mean
  building the browser yourself — on top of a desktop that costs 600–800 MB

`cog` on Wayland is also the best case for touch: the
[linux-surface wiki](https://github.com/linux-surface/linux-surface/wiki/Installation-and-Setup#touch-controls)
notes that Wayland has native touch support while X11 merely emulates a mouse pointer.

## What you need

- a Surface 3 (the Atom model) with its power supply
- a **Type Cover or a USB keyboard**. The Debian installer is text mode and does not take touch
  input, so you cannot install without a keyboard — and the single USB port holds the install stick
- a 4 GB USB stick

## Install Debian 13

1. **Write the installer** (`debian-13.7.0-amd64-netinst.iso`, ~793 MB). The Wi-Fi firmware the
   tablet needs is on the media, so the installer can configure your network.

   ```sh
   sudo dd if=debian-13.7.0-amd64-netinst.iso of=/dev/sdX bs=4M oflag=direct conv=fsync status=progress
   ```

2. **Boot the stick.** Power the tablet off, insert the stick, then hold **Volume Down** while
   pressing **Power** and keep holding Volume Down for 15–20 seconds. If that does nothing, hold
   **Power + Volume Up** for the UEFI menu, then set **USB Storage** first under _Boot configuration_.

3. **Install.** Choose _Install_, then follow the normal path with one difference: at _Software
   selection_, select **only "standard system utilities"** and no desktop environment. At _Disk
   partitioning_, pick the internal eMMC — not the 3.8 GB USB stick. Leave the root password empty so
   `sudo` uses your user.

   :::tip[Before you partition]
   Switch to a console with `Alt+F2` and check that the hardware is there:

   ```sh
   cat /proc/bus/input/devices | grep -i name     # expect "Surface3 SPI Touchscreen" + Type Cover
   grep . /sys/devices/virtual/dmi/id/* 2>/dev/null | grep OEM | grep -v -e modalias -e uevent
   ```

   Output on the second line means a broken DMI table. That is fine — the linux-surface kernel has the
   workaround — but it is why the kernel below is not optional on such a unit.
   :::

## Install the Surface kernel

```sh
wget -qO - https://raw.githubusercontent.com/linux-surface/linux-surface/master/pkg/keys/surface.asc \
    | gpg --dearmor | sudo dd of=/etc/apt/trusted.gpg.d/linux-surface.gpg

echo "deb [arch=amd64] https://pkg.surfacelinux.com/debian release main" \
    | sudo tee /etc/apt/sources.list.d/linux-surface.list

sudo apt update
sudo apt install -y linux-image-surface linux-headers-surface \
                    intel-microcode firmware-libertas firmware-sof-signed

sudo update-grub && sudo reboot
```

Three packages from the upstream instructions are **not** installed here, on purpose:

| Package                        | Why not                                                                                                                                           |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| `iptsd`                        | that is Intel Precise Touch & Stylus, for the Surface Pro 4 and newer — the Surface 3 uses `surface3-spi`                                         |
| `libwacom-surface`             | known to break recent Debian/Ubuntu systems ([#2076](https://github.com/linux-surface/linux-surface/issues/2076)) and is not needed under `labwc` |
| `linux-surface-secureboot-mok` | only needed with Secure Boot enabled                                                                                                              |

After the reboot, check that you are actually running it:

```sh
uname -r          # must contain "surface"
ls /sys/class/backlight/          # expect intel_backlight
```

The stock Debian kernel stays installed as a fallback entry in the boot menu.

## Memory and power

With 2 GB of RAM, zram is not optional:

```sh
sudo apt install -y systemd-zram-generator

sudo tee /etc/systemd/zram-generator.conf >/dev/null <<'EOF'
[zram0]
zram-size = ram / 2
compression-algorithm = zstd
EOF

sudo tee /etc/sysctl.d/99-picture-frame.conf >/dev/null <<'EOF'
vm.swappiness = 150
vm.page-cluster = 0
EOF

sudo systemctl daemon-reload && sudo reboot
```

Then turn suspend off. The Marvell Wi-Fi driver has a documented history of dying on resume
([linux-surface#1965](https://github.com/linux-surface/linux-surface/issues/1965)), and a frame has no
reason to sleep — the screen is blanked through DPMS, not by suspending the system:

```sh
sudo systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target
sudo sed -i 's/^#\?HandleLidSwitch=.*/HandleLidSwitch=ignore/' /etc/systemd/logind.conf
sudo systemctl restart systemd-logind
```

## Install the frame

```sh
curl -fsSL https://github.com/MateEke/picture-frame/releases/latest/download/install.sh \
  | sudo bash -s -- --display-output eDP-1
```

The display output is the one thing the installer cannot guess on this machine, so pass `eDP-1`. The
rest works as on a Pi: minimum requirements are a working network with NetworkManager and the Debian
packages the installer pulls in.

:::note[No amd64 release asset?]
Releases have so far shipped `armv6`, `armv7` and `arm64` tarballs only. If the release you install
has no `amd64` asset, the one-liner above fails at the download. Build the binary from source on a
faster machine with `make build-go-amd64`, copy `dist/picture-frame` plus the `deploy/` tree to the
tablet, and pre-place the payload so the installer skips the download:

```sh
# on the tablet, with dist/picture-frame, deploy/ and config.example.toml in ~/pf-staging
cd ~/pf-staging
DEST="$HOME/picture-frame"
mkdir -p "$DEST"
install -m 0755 dist/picture-frame  "$DEST/picture-frame"
install -m 0644 config.example.toml "$DEST/config.example.toml"
printf 'v1.3.0\n' > "$DEST/.release"        # must match the --version you pass

sudo bash deploy/install.sh --version v1.3.0 --display-output eDP-1 --no-ap
```

`install.sh` skips the release download when the payload is already in place and its `.release`
marker matches the requested tag. Note that the frame's self-updater will not update a platform it
finds no asset for — it logs and leaves the binary alone.
:::

## Verify

```sh
systemctl status labwc kiosk-backend kiosk-browser --no-pager
```

- **Screen power.** Blanking is the least certain part on Intel:

  ```sh
  XDG_RUNTIME_DIR=/run/picture-frame WAYLAND_DISPLAY=wayland-0 wlopm --off eDP-1
  XDG_RUNTIME_DIR=/run/picture-frame WAYLAND_DISPLAY=wayland-0 wlopm --on eDP-1
  ```

  If the panel does not respond, set `blank_after = "0s"` in `config.toml` and drive the screen from
  a brightness of zero instead.

- **Touch.** Tap left half for the previous photo, right half for the next, and once on a dark screen
  to wake it. Watch the log while you tap for a few minutes:

  ```sh
  sudo dmesg -w | grep -iE 'surface3|SPI transfer timed out'
  ```

- **Audio.** `speaker-test -c2 -twav -l1`.

## Troubleshooting

### Touchscreen

The `surface3-spi` driver has crashed randomly since kernel 5.18 with repeated
`SPI transfer timed out` messages, which leaves the screen unresponsive
([linux-surface#1184](https://github.com/linux-surface/linux-surface/issues/1184)). The linux-surface
kernel carries a workaround that disables DMA mode — the module's `use_dma` parameter, `false` by
default:

```sh
cat /sys/module/surface3_spi/parameters/use_dma     # expect N
```

If the driver still wedges, bring it back without rebooting:

```sh
sudo modprobe -r spi_pxa2xx_platform && sudo modprobe spi_pxa2xx_platform
```

Some reporters on other distributions describe touch registering only intermittently even with the
workaround. On the frame that matters less than on a desktop: the gesture is a plain tap, not a drag,
and everything else is managed from a phone.

### Wi-Fi

If the interface disappears entirely instead of merely dropping, the `mwifiex` firmware has crashed.
Reloading the module recovers it, so a small watchdog is worth installing:

```sh
sudo tee /usr/local/sbin/pf-wifi-watchdog.sh >/dev/null <<'EOF'
#!/bin/bash
# Reload mwifiex if the interface vanished (driver death, not a normal disconnect).
set -u
DEV=wlan0
if ! ip link show "$DEV" >/dev/null 2>&1; then
    logger -t pf-wifi-watchdog "$DEV gone: reloading mwifiex"
    modprobe -r mwifiex_pcie mwifiex cfg80211 2>/dev/null || true
    sleep 2
    modprobe mwifiex_pcie || true
    systemctl restart NetworkManager || true
fi
EOF
sudo chmod 0755 /usr/local/sbin/pf-wifi-watchdog.sh

sudo tee /etc/systemd/system/pf-wifi-watchdog.service >/dev/null <<'EOF'
[Unit]
Description=Picture frame Wi-Fi watchdog (mwifiex recovery)

[Service]
Type=oneshot
ExecStart=/usr/local/sbin/pf-wifi-watchdog.sh
EOF

sudo tee /etc/systemd/system/pf-wifi-watchdog.timer >/dev/null <<'EOF'
[Unit]
Description=Run the picture frame Wi-Fi watchdog every 2 minutes

[Timer]
OnBootSec=3min
OnUnitActiveSec=2min

[Install]
WantedBy=timers.target
EOF

sudo systemctl daemon-reload && sudo systemctl enable --now pf-wifi-watchdog.timer
```

### Installer hangs at network detection

Older Debian installers can stall while probing the network on this hardware. Switch to another
console with `Alt+F2`, run `killall ethdetect`, and continue without configuring the network — you can
set up Wi-Fi afterwards with NetworkManager.

### Audio

The internal codec needs `firmware-sof-signed` (installed above). If you get a single continuous tone
instead of sound, add `snd_intel_dspcfg.dsp_driver=2` to the kernel command line to fall back to the
older HDA driver.

## Remote access

The frame should stay on your own network. For access from outside, the
[security page](/manual/security/) suggests a VPN rather than a forwarded port, and Tailscale is the
lightest way to do it here — around 40–70 MB of RAM, no kernel module, so the patched Surface kernel
cannot break it:

```sh
curl -fsSL https://pkgs.tailscale.com/stable/debian/trixie.noarmor.gpg \
  | sudo tee /usr/share/keyrings/tailscale-archive-keyring.gpg >/dev/null
curl -fsSL https://pkgs.tailscale.com/stable/debian/trixie.tailscale-keyring.list \
  | sudo tee /etc/apt/sources.list.d/tailscale.list
sudo apt update && sudo apt install -y tailscale
sudo tailscale up --ssh
```

That also gives you SSH to the tablet without keeping a keyboard around, and the admin interface over
`http://pictureframe/` from anywhere on your tailnet. Do not advertise it as an exit node or subnet
router: there is no CPU to spare.
