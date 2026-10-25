#!/usr/bin/env bash
# Install the optional overnight suspend timer. Run as root on the frame:
#
#   sudo bash deploy/night-suspend/install.sh
#
# The timer is enabled by default. Pass --no-enable to install the files but
# leave the timer disabled until you have tested suspend/resume.
set -euo pipefail

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
    echo "run as root: sudo bash $0" >&2
    exit 1
fi

enable_timer=1
case ${1:-} in
--no-enable) enable_timer=0 ;;
"") ;;
*)
    echo "usage: $0 [--no-enable]" >&2
    exit 2
    ;;
esac

here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)

install -m 0755 "$here/picture-frame-night-suspend.sh" /usr/local/sbin/picture-frame-night-suspend
install -m 0644 "$here/picture-frame-night-suspend.service" /etc/systemd/system/picture-frame-night-suspend.service
install -m 0644 "$here/picture-frame-night-suspend.timer" /etc/systemd/system/picture-frame-night-suspend.timer

# Seed the schedule defaults on first install only.
if [[ ! -e /etc/default/picture-frame-night-suspend ]]; then
    install -m 0644 "$here/picture-frame-night-suspend.default" /etc/default/picture-frame-night-suspend
fi

# The Surface 3 guide masks sleep because the mwifiex driver has a history of
# failing to resume. This feature explicitly needs the suspend path, so unmask
# it. If resume turns out to be unreliable, disable the timer and re-mask.
systemctl unmask sleep.target suspend.target

systemctl daemon-reload

if ((enable_timer)); then
    systemctl enable --now picture-frame-night-suspend.timer
else
    systemctl disable --now picture-frame-night-suspend.timer >/dev/null 2>&1 || true
    echo "timer installed but not enabled; enable it with:"
    echo "  sudo systemctl enable --now picture-frame-night-suspend.timer"
fi

echo
systemctl list-timers picture-frame-night-suspend.timer --no-pager || true
echo
echo "schedule: /etc/default/picture-frame-night-suspend"
echo "logs:     journalctl -t picture-frame-night-suspend -b"