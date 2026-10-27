#!/bin/sh
# Launch cog sized to the panel's native resolution.
#
# labwc fullscreens cog via a windowRule only when the panel is on. If cog
# (re)starts while the panel is DPMS-off, labwc can't resolve the output and
# cog falls back to 1024x768, stuck that way after wake. Setting
# COG_PLATFORM_WL_VIEW_WIDTH/HEIGHT bypasses the windowRule dependency, the
# connector reports its mode even while DPMS-off, and detection at launch means
# no stale value. Falls back to cog's default if detection fails.
set -eu

for conn in /sys/class/drm/card*-*/status; do
    [ "$(cat "$conn" 2>/dev/null)" = "connected" ] || continue
    mode="$(head -n1 "$(dirname "$conn")/modes" 2>/dev/null || true)"
    case "$mode" in
    [0-9]*x[0-9]*)
        COG_PLATFORM_WL_VIEW_WIDTH="${mode%x*}"
        COG_PLATFORM_WL_VIEW_HEIGHT="${mode#*x}"
        export COG_PLATFORM_WL_VIEW_WIDTH COG_PLATFORM_WL_VIEW_HEIGHT
        break
        ;;
    esac
done

# A 90/270 transform makes the logical output portrait but the pinned mode above
# is the native landscape WxH; swap so cog matches. Missing file = no rotation.
rot="$(cat "${XDG_RUNTIME_DIR:-/run/picture-frame}/rotation" 2>/dev/null || true)"
if [ -n "${COG_PLATFORM_WL_VIEW_WIDTH:-}" ]; then
    case "$rot" in
    90 | 270)
        swap="$COG_PLATFORM_WL_VIEW_WIDTH"
        COG_PLATFORM_WL_VIEW_WIDTH="$COG_PLATFORM_WL_VIEW_HEIGHT"
        COG_PLATFORM_WL_VIEW_HEIGHT="$swap"
        ;;
    esac
fi

# --webprocess-failure=restart recovers a renderer crash in place (no remap/flash).
exec /usr/bin/cog -P wl --webprocess-failure=restart http://localhost:80/kiosk
