#!/usr/bin/env bash
# Suspend the frame until the next morning. Installed as
# /usr/local/sbin/picture-frame-night-suspend and started by
# picture-frame-night-suspend.timer.
#
# The timer fires every 30 minutes inside the night window, so a failed or
# inhibited suspend gets another chance. The RTC alarm is armed *before* the
# suspend; without a working alarm the script refuses to suspend.
set -euo pipefail

# Read schedule from /etc/default/picture-frame-night-suspend (written at
# install time from the Go config; can be edited directly or via the web UI
# after a restart).
CONF=/etc/default/picture-frame-night-suspend
# shellcheck source=/dev/null
[[ -r $CONF ]] && . "$CONF"

WAKE_AT=${WAKE_AT:-07:00}
SUSPEND_START=${SUSPEND_START:-23:00}
SUSPEND_END=${SUSPEND_END:-06:30}
RTC=${RTC:-/dev/rtc0}
RTWAKE=${RTWAKE:-/usr/sbin/rtcwake}

log() { logger -t picture-frame-night-suspend -- "$*"; }

to_minutes() {
    local hh=${1%%:*} mm=${1##*:}
    echo $((10#$hh * 60 + 10#$mm))
}

now_minutes=$((10#$(date +%H) * 60 + 10#$(date +%M)))
start_minutes=$(to_minutes "$SUSPEND_START")
end_minutes=$(to_minutes "$SUSPEND_END")

# Only suspend inside the configured window (it normally crosses midnight).
if ((start_minutes < end_minutes)); then
    ((now_minutes >= start_minutes && now_minutes <= end_minutes)) || exit 0
else
    ((now_minutes >= start_minutes || now_minutes <= end_minutes)) || exit 0
fi

wake_epoch=$(date -d "today $WAKE_AT" +%s)
now_epoch=$(date +%s)
if ((wake_epoch <= now_epoch + 60)); then
    wake_epoch=$(date -d "tomorrow $WAKE_AT" +%s)
fi

# Replace any stale alarm, then arm the wake time. If this fails, do not
# suspend: there would be no automatic way back.
"$RTWAKE" -d "$RTC" -m disable >/dev/null 2>&1 || true
"$RTWAKE" -d "$RTC" -m no -t "$wake_epoch" >/dev/null

log "suspending; wake alarm at $(date -d "@$wake_epoch" '+%F %T %Z')"
if ! systemctl suspend; then
    "$RTWAKE" -d "$RTC" -m disable >/dev/null 2>&1 || true
    log "suspend failed; cleared wake alarm"
    exit 1
fi
# Do not clear the alarm here: systemctl may return as soon as the suspend is
# queued, and the kernel consumes the alarm on wake. Clearing it here could
# cancel the wake-up before the system actually suspends.
log "suspend requested"