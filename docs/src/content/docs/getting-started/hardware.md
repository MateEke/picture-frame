---
title: Hardware
description: How to build the frame from a salvaged laptop screen, a driver board, and a Raspberry Pi.
---

The software runs on any screen the Pi reaches over HDMI, so a spare monitor or a small HDMI
display works. The build this project was made for, and the one shown on the
[story page](/development/story/), is a salvaged laptop panel in a frame. That route takes a little
more hardware, and this page covers it.

## The screen and driver board

A laptop panel has no HDMI input. It exposes an LVDS connector that carries both the power and the
video signal, so you cannot drive it from the Pi directly. An LVDS controller board bridges the two:
it takes HDMI in, drives the panel over LVDS, and powers it.

The board has to match your panel. You can buy one pre-flashed for a specific panel model, or a
universal board you configure yourself. Search for your panel's model number together with "LVDS
controller board". They cost about $25 to $30 on Amazon, eBay, and the like. (Newer panels use eDP
rather than LVDS, and the board has to match that too.)

The controller takes its own power, usually a 12 V brick. Mine exposes 5 V pins on the board, so I
soldered a micro-USB cable to them and run the Pi from the same supply, with no second adapter. That
part is optional, and you can do it without soldering.

## What you'll need

- a salvaged laptop screen
- an LVDS controller board that matches the panel
- a power brick for the board, usually 12 V
- a micro-HDMI to HDMI cable
- a micro-USB cable to power the Pi
- a Raspberry Pi Zero W, Zero 2 W, or Pi 3

## My build

The controller board is an M.NT68676.2. A framer mounted the panel in an ordinary picture frame with
a small cut-out at the back for the LVDS connector, and the controller board and the Pi are fixed to
the back of the frame. Once it is assembled, the only cable leaving the frame is power.

## Sensors

The motion, temperature, and humidity readings are optional, and the hardware is up to you. If you
already run anything that speaks MQTT, such as a device in Home Assistant or on Zigbee2MQTT, the
frame reads it with no extra parts. It also talks to Bluetooth sensors directly, but only ones with
a connected mode, not the cheap broadcast-only kind. See [Sensors](/manual/sensors/#sensor-types)
for the supported types and how to set them up.
