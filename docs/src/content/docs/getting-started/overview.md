---
title: Overview
description: What the frame is, what you need to build one, and the optional pieces you can add.
---

A self-hosted picture frame for the Raspberry Pi. It runs a photo slideshow alongside the time,
the weather, and the temperature and humidity in your rooms, read from your own sensors. It can
switch the screen off when no one is around, and it connects to Home Assistant. Everything runs
on the device, with no cloud account and no telemetry.

It doesn't have a clever name. It's a picture frame.

## What you'll need

- **A Raspberry Pi.** The Zero W and Zero 2 W are the main targets, picked for their small size
  and low power, and the Zero 2 W is the one to get. A Pi 3 works too and is tested. The Pi 4
  and 5 are untested, and overkill for what this does. The installer detects the CPU (armv6,
  armv7, or arm64) on its own.
- **A screen on HDMI.** A salvaged laptop panel on a driver board is very much the point. See
  [Hardware](/getting-started/hardware/) for how to drive one.
- **Raspberry Pi OS Trixie, the _Lite_ image** (no desktop). The frame runs its own kiosk stack,
  and the desktop image ships a display manager that fights it for the screen. Use 32-bit (armhf)
  on the Zero and Zero 2 W. 64-bit is fine on the Pi 3.
- **A reachable Pi.** Flashed, booted, on your network (wired or Wi-Fi), and open to SSH or a
  console login. NetworkManager (the Pi OS default) must be running.

## Nice to have

None of these are required. Each one adds a capability:

- **A motion sensor** (Bluetooth, or anything that reports through MQTT), so the screen sleeps
  when the room empties and wakes when you return.
- **Temperature and humidity sensors** (Bluetooth or MQTT), so the overlay can show what it's
  actually like inside and out.
- **An Immich shared-album link**, to pull photos straight from [Immich](https://immich.app)
  instead of uploading them by hand.
- **An MQTT broker**, to join the frame to [Home Assistant](https://www.home-assistant.io).

You can add any of these later from the admin interface. Nothing here has to be decided up front.

## Next

[Install it](/getting-started/install/) on the Pi, then learn the
[configuration basics](/getting-started/configuration/).
