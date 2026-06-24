---
title: The kiosk display
description: What the frame shows on its own screen, the photo with the time, date, and your room readings.
---

The kiosk is what the frame shows on its own screen: a photo filling the display, with the time,
date, and your room readings along the bottom. There is nothing to configure here. Everything on
the screen is driven by the settings elsewhere in this manual, and the frame shows the
result.

![The frame showing a photo, with the time, date, and indoor and outdoor readings along the bottom](../../../assets/screenshots/kiosk.png)

## What is on screen

**The photo.** The current slideshow image fills the screen and advances on the interval you
set. Manage the photos on [Photos](/manual/photos/), and set the timing and order on
[Slideshow & display](/manual/slideshow-display/). With no photos added yet, the screen shows
just the clock and date.

**The clock and date.** The time sits large in the corner, with the weekday and date beneath it,
formatted for the language you chose, in 12- or 24-hour style, and in the time zone you set. You
can also hide the clock and date entirely. Set all of this under
[Slideshow & display](/manual/slideshow-display/).

**The readings.** Along the bottom the frame shows, when each one is available, the outside
reading, the inside reading, and the humidity, each under the caption you gave it:

- the **outside** reading comes from an outside temperature [sensor](/manual/sensors/), or from
  [weather](/manual/weather/) when no such sensor is set, and the weather icon appears when weather
  is on;
- the **inside** temperature and **humidity** come from the matching inside sensors.

A reading shows only when its source is configured, and one that has stopped arriving falls back
to `--`. When nothing is left to show, no readings, no weather, and the clock and date hidden, the
whole bottom overlay disappears and the photo fills the screen on its own.

## When the screen sleeps

With a motion sensor, the frame turns the screen off when the room empties and wakes it when you
return. See [Slideshow & display](/manual/slideshow-display/). You can also switch it on or off
by hand from the [Dashboard](/manual/dashboard/) or [Home Assistant](/manual/home-assistant/).
