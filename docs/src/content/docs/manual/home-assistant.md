---
title: Home Assistant
description: Join the frame to Home Assistant over MQTT, publishing its sensors and a screen switch with no YAML.
---

The frame joins Home Assistant as a proper device over MQTT. It publishes its sensor readings
and a screen switch to your broker, and Home Assistant discovers them on its own, with no YAML
to write. Set it up under **Settings → Home Assistant**.

![The frame as a device in Home Assistant: a Screen switch, inside temperature, humidity, and motion, screen power, and related automations](../../../assets/screenshots/home-assistant.png)

## What you need

- An **MQTT broker** on your network, such as the Mosquitto add-on.
- Home Assistant's **MQTT integration** with discovery enabled, which is the default.
- At least one [Bluetooth sensor](/manual/sensors/) if you want readings. The screen switch is
  published either way.

## Connecting to the broker

Under **Settings → Home Assistant**, fill in the connection:

- **Broker**, as `tcp://` or `ssl://` with a host and port, for example
  `tcp://192.168.1.10:1883`.
- **Client ID**, unique to this frame.
- **Username** and **Password**, if your broker requires them.

This same broker also feeds [MQTT sensors](/manual/sensors/), so it is not tied to Home
Assistant, and readings can come from any source on the broker. Once a broker is set, turn on the
**Home Assistant bridge**.

## What it exposes

With the bridge on, Home Assistant discovers one device carrying:

- each Bluetooth sensor's readings, temperature and humidity as sensors, motion as a binary
  sensor;
- a **Screen** switch, to turn the panel on or off from Home Assistant;
- a read-only **screen-power** sensor, showing whether the panel is actually lit.

Only Bluetooth sensors are bridged. Mock sensors are a development aid, and MQTT sensor readings
already come from the broker, so re-publishing them would loop.

:::tip[Showing Home Assistant's own data on the frame]
The bridge sends data to Home Assistant. To go the other way and show a Home Assistant entity
on the frame, enable Home Assistant's
[MQTT Statestream](https://www.home-assistant.io/integrations/mqtt_statestream/) integration so
it publishes entity states to the broker, then read them back as [MQTT sensors](/manual/sensors/).
:::

The switch reflects intent, not the live panel. Motion and idle blanking move the screen-power
sensor but leave the switch alone. Turning the switch on re-enables motion auto-wake. Turning it
off forces the screen off and holds it there. With no motion sensor, it is a plain on-and-off.
See [Slideshow & display](/manual/slideshow-display/).

## Bridge settings

- **Node ID** is the device's identity in Home Assistant, its device id and the prefix for
  every entity. Give each frame a distinct value if you run more than one.
- **Base topic** namespaces the state, command, and availability topics.
- **Discovery prefix** must match Home Assistant's MQTT discovery prefix, `homeassistant` by
  default.
- **Stale after** marks a sensor unavailable in Home Assistant when no reading arrives within
  the window. An entity shows as available only while both the frame and that sensor are online,
  so a dead frame and a silent sensor both surface honestly instead of leaving a stale value
  looking live.

:::note[Restart required]
The broker connection and the bridge settings take effect after the frame restarts.
:::

## Using it

The device appears under **Settings → Devices & services → MQTT** in Home Assistant. From there
you can add its entities to dashboards, or fold them into automations: blank the screen when you
leave, wake it when you are back, or use the frame's motion and climate readings elsewhere in
your home.

Every setting on this page maps to a key in the [configuration reference](/reference/configuration/).
