---
title: Slideshow & display
description: Set how photos advance, when the screen sleeps, the language on the frame, and which screen-power backend it uses.
---

The slideshow and the screen's behavior are set on the **Settings** page, in two groups.
**Essentials** holds the photo rotation, the idle screen-off, the language, and the reading
labels. These apply the moment you save. **Display** holds the screen output and which
screen-power backend the frame uses. See [Configuration basics](/getting-started/configuration/)
for how saving and restarts work.

![The Settings page, with the Essentials group showing the slideshow and display controls](../../../assets/screenshots/settings.png)

## The photo rotation

**Advance photo every** sets how long each photo stays on screen before the next one. The
default is two minutes (`slideshow.interval`).

**Shuffle photos** controls the order. With it off, photos cycle in order. With it on, the order is
reshuffled each full pass (`slideshow.randomize`, off by default).

## Turning the screen off when idle

**Turn screen off when idle** blanks the panel after a stretch with no motion, and motion
wakes it again (`display.blank_after`, twenty minutes by default). This is a true power-off of
the panel, not a black photo.

It needs a motion sensor. Without one the control reads **Never** and is disabled, since
nothing would be left to wake the screen. To turn the screen on and off by hand instead, use
the toggle on the [Dashboard](/manual/dashboard/) or the switch exposed to
[Home Assistant](/manual/home-assistant/).

:::note[Some TVs stay lit]
A monitor or a laptop panel cuts its backlight when the screen blanks. Some TVs ignore the
signal and stay lit on a black screen. Powering a TV down fully takes HDMI-CEC, which is
outside what the frame controls.
:::

## Language and labels

**Language** sets the locale the frame formats with, both the date wording and the 12- or
24-hour clock (`display.locale`, American English by default). It changes the frame, not the
admin interface.

**Reading labels** are the captions under the sensor readings on the frame, in your own words:
the outside reading, the inside reading, and humidity. Leave one blank to hide that caption.
[The kiosk display](/manual/kiosk/) shows where they appear.

## Screen-power backend

:::note[Advanced]
The default backend is the right choice for almost every setup. Change it only if you have a
specific reason to.
:::

The **backend** is how the frame powers the panel. Two options exist: **wlopm** (default,
recommended) and **vcgencmd** (a legacy fallback). wlopm is the more reliable path. vcgencmd
trims a little memory but can be less stable, so use it only if wlopm gives you trouble.

Each backend installs its own system services and boot configuration, so the **Backend**
dropdown in Settings does not switch between them on its own. To change the backend, re-run the
installer with its `--display-backend` flag (see [Install](/getting-started/install/)). It
reconfigures the system, and you reboot to apply. [The story & the hard parts](/development/story/)
covers why the two differ.

## Screen output

**Wayland output** is the display connector the wlopm backend targets, such as `HDMI-A-1`. Pick
a connected display from the list or type the name.

:::note[Restart required]
A change to the screen output takes effect after the frame restarts.
:::

Every setting on this page maps to a key in the [configuration reference](/reference/configuration/).
