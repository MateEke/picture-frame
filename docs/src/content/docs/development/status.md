---
title: Project status
description: What this project is, how it's maintained, and what to expect from it.
---

This is a small, niche project, built mostly around my own needs. I've run it, or rather its
earlier iterations, in my living room for nearly ten years.

## Maintenance

I'll keep up with security patches and dependency updates. I don't have plans for big new
features, though. It does what I need, and it has for years.

## Why it's open source

I'd meant to release it for a long time. Part of the reason is selfish: knowing other people
might read the code made me think harder about quality. The other part is that a few of the
problems I worked out here could be useful elsewhere. The slideshow fader is the clearest
example. It took a lot of fiddling before the crossfade looked right on the original Pi Zero,
and that work might save someone else the trouble. There's more on that in
[the story](/development/story/).

## Contributing

If you're using it and want something it doesn't do, open an issue. If it's feasible, I'm happy
to build it. I also welcome pull requests, and I'll merge the ones that are good quality and
don't degrade the core behavior. See [Contributing](/development/contributing/) for how to set up a
development environment.

## Maybe later

A few things I might get to, with no promises on timing:

- **Localization for the admin pages.** English today, with Hungarian next on my list.
- **CEC display power.** I'd like to try switching a TV on and off over HDMI-CEC, though in my
  experience CEC tends to be too finicky to lean on for this.
- **Showing photos uncropped.** Photos that don't match the screen's shape are cropped to fit
  today. I'd rather show them whole on a matching backdrop, the way the
  [dont-crop](https://github.com/jwagner/dont-crop) library fits a gradient to an image. Doing
  that on the kiosk is probably too much for the Pi, so I'd first need to generate the backdrop
  earlier, at upload or sync time.
- **Wired GPIO sensors.** A way to read motion, temperature, and humidity straight off the Pi's
  GPIO, over something like I2C, so a unit can be fully self-contained with no broker or smart-home
  setup. Bluetooth lets me put the motion sensor anywhere in the room, which helps a frame set back
  on a shelf, but it is more to build, needs a battery in the end, and is less reliable. Wired
  sensors trade that placement freedom for a simpler, sturdier build. People who visit keep asking
  where to get a frame like this, and while it doesn't make sense as a product (a good screen isn't
  cheap, framing costs more, and salvaged panels aren't a reliable supply), I'd like to build a few
  simple units for less technical friends and family.
