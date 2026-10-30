# Video playback on the slideshow (Surface 3) — evaluation

**Verdict:** feasible, but only as a narrow feature. Not tested on a Surface 3. The hardware and codec
claims come from general knowledge of the chip and from reading the repo, so verify them on the device.

## What blocks it today

- **Videos are filtered out at every layer.**
  - Local files accept only `.jpg`, `.jpeg` and `.png` (`internal/library/adapter/fs.go:43`).
  - The Immich sync drops anything where `IsImage` is false (`internal/library/adapter/immich/immich.go`,
    around line 334).
  - Immich fetches the preview-size thumbnail, never the original (`Fetch`, `/thumbnail?size=preview`).
  - Uploads are decoded as images and re-encoded as JPEG (`internal/httpapi/images.go`).
- **The kiosk only knows images.** `web/src/lib/Slide.svelte` renders `<img src="/img/…">` and waits on
  `img.decode()`. The `Fader` crossfade and the `next` preloading in
  `web/src/routes/kiosk/components/Images.svelte` both assume that.
- **The slide plan has no notion of duration.** `internal/slideplan` plans slides by aspect ratio and
  pairs them side by side. A clip needs its own duration, or "advance when it ends".

## Surface 3 constraints

- **CPU and RAM.** Atom x7-Z8700 with 2 GB of RAM. The memory budget in
  `drafts/surface3-debian13-pictureframe.md` leaves about 1 GB free. That is enough for one 1080p H.264
  stream, but not for two decoders during a crossfade.
- **Hardware decode.** Cherry Trail's Gen8 GPU should decode H.264 in hardware over VA-API. It has no
  VP9 or AV1 hardware decode, and HEVC is doubtful. Software decode of 1080p on this CPU will stutter and
  run hot. Restrict clips to H.264, 1080p or lower, capped at 30 fps.
- **The browser is the main risk.** The kiosk runs `cog -P wl` (`deploy/kiosk-browser-launch.sh`), which
  is WebKit on GStreamer. The installer and docs never mention GStreamer, `gstreamer1.0-libav` or
  `gstreamer1.0-vaapi`. Whether `<video>` plays at all, and whether it is hardware accelerated, depends on
  those packages and on the Debian 13 WebKit build.
- **Autoplay.** `<video muted playsinline autoplay>` is allowed to autoplay.
- **Power and eMMC.** Continuous decode keeps the CPU awake and the panel lit. Motion-based screen
  blanking still works as before. Cached clips take eMMC space.

## Proposed design

1. **Library and sync.**
   - Accept `.mp4` (H.264) locally. For Immich, accept `isImage=false` assets of type VIDEO.
   - Fetch the original, or Immich's transcoded `/video/playback`, and cache it on disk like the images.
   - Add a media-kind field to `library.Asset` and `Image`, and enforce a size and duration cap.
2. **Slide plan.**
   - A video slide is always solo, never paired, so only one decoder is active at a time.
   - Its duration is the clip length, clamped to a maximum.
3. **Kiosk.**
   - Add a `<video>` branch in `Slide.svelte` with `preload="auto"` and `muted`.
   - Treat `canplaythrough` as the load signal in place of `decode()`.
   - Skip or shorten the crossfade into and out of a video. The frame already avoids heavy animation on
     weak hardware (see the Pi Zero comment in `Images.svelte`).
   - Don't preload the next clip while one is playing.
4. **Fallback.** If the video errors, or `canPlayType('video/mp4; codecs="avc1.640028"')` returns an
   empty string, skip the slide. Do not show a black frame.

## Rough effort

- Backend library, sync and plan changes: about 2–3 days with tests.
- Kiosk rendering and fader changes: about 1–2 days.
- On-device codec and performance tuning on the Surface: unknown. It could be hours, or it could kill
  the idea.

## Recommended next step: 30-minute spike

Before writing any feature code, on the Surface:

1. Install `gstreamer1.0-libav` and `gstreamer1.0-vaapi`.
2. Serve a test H.264 1080p30 clip through cog at `/kiosk`.
3. Watch three things:
   - whether it plays at all
   - CPU use during playback (`top`, and `intel_gpu_top` if installed)
   - whether it stutters during a crossfade

If it plays smoothly with CPU under about 50%, build it as described above. If not, the "lightweight
frame" goal says to drop it, or to offer only animated WebP or GIF loops instead.

## Alternative: play video with MPV

MPV is a stronger option for the decoding itself and a weaker one for fitting into the current frame.
Not tested on the device.

### What MPV improves

- **Playback reliability.** It uses ffmpeg and VA-API directly, with no WebKit or GStreamer plugin chain
  in between. That chain is the main risk of the in-page `<video>` approach. `mpv` is in Debian 13
  `main`, and `--hwdec=vaapi` is its best-supported path on Intel.
- **Codec coverage.** If hardware decode fails, it falls back to software. That would still stutter on
  this Atom CPU, but it won't refuse to play the way a missing GStreamer plugin would.
- **Low overhead.** `--vo=dmabuf-wayland` (or `--vo=gpu --gpu-context=wayland`) with hardware decode
  should be light on 2 GB of RAM.
- **Control.** It has a JSON IPC socket, so the Go backend can load a file, wait for end-of-file, and
  quit.

### What MPV costs

- **A second process and a second window.** The clock, weather and sensor overlay are web content inside
  cog, and mpv's fullscreen window would cover them. Either accept a video-only mode with no overlay, or
  re-draw the overlay in mpv (OSD or a script). Layering cog on top of mpv with transparency is not
  realistic with cog and WPE.
- **Window management.** A `labwc` window rule would need to make mpv fullscreen and on top, and send it
  away afterwards. `deploy/kiosk-browser-launch.sh` already documents labwc misbehaving with cog's size
  at DPMS-off, so expect similar edge cases. Rotation is handled by labwc, so it should carry over.
- **Touch navigation.** `TouchNav` lives in the page. While mpv has focus, taps go to mpv, so it needs
  `--no-input-default-bindings` and a way to send taps back to the backend, such as an mpv input command
  that calls the existing next/previous API.
- **No crossfade.** The transition into and out of a video is a cut, or a fade through black.
- **Lifecycle.** The backend (or a new systemd unit) must start and stop mpv, handle crashes, and respect
  the screen-power policy. The installer must add the `mpv` package.

### Comparison

| | In-page `<video>` | MPV |
|---|---|---|
| Overlay, touch, crossfade | Reused as is | Lost or reimplemented |
| Decode reliability on Surface 3 | Uncertain (WebKit and GStreamer) | Good (ffmpeg and VA-API) |
| Code changes | Library, plan, `Slide.svelte` | Same, plus process and window orchestration |
| New moving parts | Packages | Process, IPC, window rules |

### Recommendation

Run the spike with both. The in-page test is described above. The MPV test is:

```sh
mpv --hwdec=vaapi --vo=dmabuf-wayland --fs clip.mp4
```

run under labwc, watching CPU use.

- If the in-page `<video>` plays well, use it. It keeps the whole experience in one page.
- If it doesn't, MPV is the fallback. Design the video slide as a full-screen, no-overlay mode and
  accept the cut transitions.
