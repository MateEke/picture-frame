# Running other apps (working note)

The frame is built to own the screen. This is the case where it shouldn't: you want one other app
on the display — a dashboard, a weather page, a control panel — and a way to switch to it and
back. This is a design note, not a shipped feature: nothing here is part of the installer or the
admin interface.

The model assumes a **single visible app at a time**, the way the frame already works. The
question is only *which* app that is, and how to get back to the frame.

## The frame assumes it is the only thing running

Two properties decide everything that follows.

- **The kiosk browser must keep beating.** Its page posts `/api/heartbeat` every 20 seconds
  (`web/src/lib/heartbeat.ts`), and the backend exits on its own if no beat arrives for five
  minutes (`cmd/picture-frame/main.go`). Because `kiosk-browser.service` is
  `PartOf=kiosk-backend.service`, the restart that follows drags the browser back with it.
- **The backend owns panel power.** Idle-blank, motion-wake, and the manual switch all live in
  `internal/display/policy.go`, independent of who is drawing on screen.

The consequence is blunt: stop the frame's browser to show something else and, within five
minutes, the frame restarts itself over your app. Pointing the browser at a different URL has the
*same* effect — the kiosk page unmounts, so the heartbeats stop. Whatever "switch app" means here,
it has to keep the frame's own page alive, or replace its liveness signal.

## An app is one fullscreen client

On this device the kiosk stack is a Wayland compositor (labwc) plus one client (cog, the browser)
pointing at the frame. A second app is therefore naturally either:

- **a web view** — another page shown by the existing browser, or
- **a separate Wayland client** — its own process, connecting to the same compositor.

Both are fed by the same labwc config, whose wildcard window rule fullscreens any toplevel
(`deploy/labwc/rc.xml`), so neither needs compositor changes to fill the screen, and both need the
kiosk environment (`XDG_RUNTIME_DIR=/run/picture-frame`, `WAYLAND_DISPLAY=wayland-0`, seat access)
to draw at all.

## The verdict: web views in the existing browser

For one app at a time on a 2 GB tablet, this is not close.

| | Web views in the existing browser | Separate Wayland clients |
| ------------------- | ---------------------------------- | ---------------------------------------- |
| Processes           | **one browser**                    | two, or a duplicated WebKit on top       |
| RAM                 | roughly the current footprint      | +150–250 MB per browser-class app        |
| Switching           | swap the view                      | start/stop units, compositor semantics   |
| Frame's heartbeat   | **survives on its own**            | dies when you leave, then restart cascade |
| Kinds of app        | web UIs only                       | anything                                 |

The last two rows are what decide it. With separate clients and one-app-at-a-time you *stop* the
frame's browser to show the other app, and the heartbeat rule above brings the frame back minutes
later. Avoiding that means either leaving the frame mounted underneath — which is multitasking,
the thing we ruled out — or stopping the whole frame stack, and then losing the admin interface
and Home Assistant while the other app is up.

Separate clients are only the right answer when the app genuinely is not a web page: a terminal,
an emulator, a media player. Even then, the honest framing is that the frame is *off*, not
backgrounded, while it runs.

## Getting the heartbeat right

Switching views is easy; the heartbeat is the part that bites. A host page that never unmounts is
what keeps the frame alive. There are two ways to build it.

1. **Keep the frame's view mounted (preferred).** A small local host page keeps the frame's kiosk
   view loaded but hidden, and swaps the active app's view in front of it. The frame's own page
   keeps beating, untouched, so nothing about the frame is weakened — including update rollback.
2. **Post a keepalive yourself (simpler, lossy).** A timer POSTs `/api/heartbeat` so the backend
   never exits while any app is shown, and you can unload the frame's view entirely. The cost is
   that during a self-update this can satisfy the commit gate (`watchUpdateCommit`, `main.go`)
   with a build that never actually rendered, defeating rollback. If you take this route, pause
   the keepalive while an update is in flight.

Prefer the first: it is the only version where "one app visible" is true without weakening the
frame.

**Frames and X-Frame-Options.** Hosting the frame and the other app as views means iframes. Web
UIs that send `X-Frame-Options: DENY` or `SAMEORIGIN` — some dashboards do — will refuse to embed.
Serve the other app through a small local reverse proxy that strips those headers and pins it to
the same origin; everything already runs on `localhost`, so this stays offline.

## Panel power

Once a second app can be on screen, the frame should no longer be the one blanking the panel, or
it will darken the other app on its own schedule. Set `display.blank_after = "0s"` in
`config.toml` and move blanking to whatever switches apps, driving the output directly:

```sh
XDG_RUNTIME_DIR=/run/picture-frame WAYLAND_DISPLAY=wayland-0 wlopm --off eDP-1
XDG_RUNTIME_DIR=/run/picture-frame WAYLAND_DISPLAY=wayland-0 wlopm --on  eDP-1
```

See the frame-side settings in the docs (Slideshow & display), and the Surface 3 page for the
Intel `eDP` caveats around `wlopm`.

## Decision rule

- **The other apps are web pages** (dashboard, weather, Home Assistant, a custom panel) → **use
  web views in the existing browser.** This is almost always the case, and it is strictly better
  on this hardware.
- **You need an app that is not web** → a separate Wayland client, accepting that entering it
  means stopping the frame, not overlaying it.
