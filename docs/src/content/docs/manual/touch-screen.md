---
title: Touch screen
description: The on-device menu for a touch panel such as the Raspberry Pi Touch Display 2 — gallery, uploads, files, sleep mode and settings.
---

With a touch panel on the Pi, the frame's own screen is more than a slideshow. A tap opens a
menu (in Finnish) with five views, and after a while without a touch the slideshow takes over
again. A remote browser opening `/kiosk` still only sees the slideshow.

## Getting around

- **In the slideshow**, tap anywhere to open the menu. Swipe left or right to change the photo.
  When the screen is dark, the first tap only wakes it.
- **In the menu**, the tab bar at the bottom switches views. "Aloita diaesitys" on the home view
  starts the slideshow at once; otherwise it starts by itself after `sleep.idle_after` (2 minutes by
  default, `0s` to never).

## The views

| View          | What it does                                                                                                                                                                    |
| ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Koti**      | Clock, weather and readings, photo and file counts, the night schedule, and buttons to start the slideshow or turn the screen off.                                             |
| **Galleria**  | Every photo as a thumbnail. Tap to view full screen and swipe through. "Valitse" (or a long press) selects several to hide from or show in the slideshow, or to delete.        |
| **Lähetä**    | A QR code and address for the admin Photos page. Open it on a phone on the same network and pick or drop photos and files, many at once. New photos appear here as they land. |
| **Tiedostot** | Videos, PDFs and other non-photo uploads. Videos, audio, images and text preview on the screen; anything else shows a QR code to open it on a phone.                           |
| **Asetukset** | Slideshow speed, shuffle and split screen, the sleep delay, the night schedule, brightness and rotation. Changes save themselves.                                              |

Hidden photos stay in the library and in the admin Photos page; they are only skipped by the
slideshow. If every photo is hidden, the slideshow shows them all rather than a black screen.

## Uploading

Photos go through the same pipeline as before: the browser shrinks them to the frame's size before
sending, so a phone full of 12 MP pictures uploads quickly. Anything that isn't a photo (or a photo
the browser can't decode, such as HEIC outside Safari) is stored as-is under Files instead of being
refused. The admin UI also has its own **Files** page.

The touch screen's own views work without the admin password, but only from the frame itself
(loopback). Reboot, power off, and the full settings stay behind the password.

## Night schedule

With `sleep.schedule` on, the screen turns off between `off_from` and `off_until` (for example
23:00–07:00, wrapping past midnight). A touch lights it for `wake_for`; motion sensors don't wake it
inside the window. In the morning it turns back on by itself.

## Raspberry Pi 4 with Touch Display 2

Build the arm64 binary on your computer and copy the checkout (or at least `dist/`, `deploy/` and
`config.example.toml`) to the Pi:

```sh
make build-pi4                 # UI + dist/picture-frame-arm64
rsync -a --exclude node_modules --exclude web/node_modules ./ pi@frame.local:picture-frame-src/
```

On the Pi (64-bit Raspberry Pi OS):

```sh
cd ~/picture-frame-src
sudo bash deploy/install.sh --local-binary dist/picture-frame-arm64 --display dsi
sudo reboot
```

`--display dsi` adds the Touch Display 2 overlay (`vc4-kms-dsi-ili9881-7inch`; use
`--touch-display-size 5` for the 5" panel), skips the HDMI pin that would create a phantom output,
and installs a udev rule so the frame can set the backlight. `--local-binary` also turns nightly
auto-update off, because a stock release would replace your build.

The panel is portrait (720×1280) by default; set **Kierto** in the touch settings for landscape.
