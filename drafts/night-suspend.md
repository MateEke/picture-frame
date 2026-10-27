# Night suspend + morning resume — idea + implementation

## Goal

Suspend the frame automatically at night and bring it back in the morning without
manual intervention.

## Core mechanism

The important part is that a morning systemd timer cannot wake a suspended
machine, because timers do not run while the system is suspended. The wake-up
must be an **RTC wake alarm armed before suspending**.

Proposed flow:

1. A systemd timer fires at night (for example 23:00).
2. A small root script calculates the next morning wake time.
3. The script writes that timestamp to the RTC wake alarm.
4. The script runs `systemctl suspend`.
5. The RTC wakes the machine at the configured morning time.
6. The timer retries every 30 minutes during the night window, so a failed or
   inhibited suspend gets another chance.

This is OS-level. It does not require changes to the slideshow/backend.

## What was implemented

### Config: `suspend_schedule` (`internal/config/config.go`)

- **Type**: `SuspendScheduleConfig` with fields:
  - `enabled` (bool) — if false, no automatic suspend
  - `suspend_start` (string, HH:MM) — e.g. "23:00"
  - `suspend_end` (string, HH:MM) — e.g. "06:30"
  - `wake_at` (string, HH:MM) — e.g. "07:00"
- **Default** (in `defaults()`): `enabled=true, suspend_start="23:00", suspend_end="06:30", wake_at="07:00"`
- **TOML key**: `suspend_schedule` in `runtime-overrides.toml`
- **Validation**: all time strings must match `HH:MM` when `enabled=true`

The config is exposed via the web UI (Settings → "Suspend schedule") and can also
be edited directly in `runtime-overrides.toml`.

### Script: `/usr/local/sbin/picture-frame-night-suspend`

- **File**: `deploy/night-suspend/picture-frame-night-suspend.sh`
- **Behavior**:
  - Reads schedule from `/etc/default/picture-frame-night-suspend` (written at
    install time from the Go config; can be edited directly or overridden via the
    web UI after a restart).
  - Checks the current time is inside the `suspend_start`–`suspend_end` window
    (crosses midnight by default 23:00–06:30). Outside the window, the script
    exits immediately (no-op).
  - Computes the next wake epoch (today's `wake_at` if still ahead, otherwise
    tomorrow's).
  - Arms the RTC wake alarm via `rtcwake -m no -t <epoch>` (clears any stale
    alarm first).
  - Runs `systemctl suspend`. The script blocks until the frame resumes.
  - Does **not** clear the alarm on resume — the kernel consumes it when the
    RTC fires.
  - If `systemctl suspend` fails, the alarm is disabled and the script exits with
    code 1.

### Systemd: timer + service

- **Timer**: `deploy/night-suspend/picture-frame-night-suspend.timer`
  - Fires every 30 minutes from 23:00 through 06:30
    (`OnCalendar=*-*-* 23,00,01,02,03,04,05,06:00/30:00`).
  - Only the script runs inside the window; a manual start outside it is a no-op.
  - `Persistent=false` so a missed tick does not immediately suspend.
- **Service**: `deploy/night-suspend/picture-frame-night-suspend.service`
  - `Type=oneshot`, `ExecStart=/usr/local/sbin/picture-frame-night-suspend`
  - `TimeoutStartSec=0` (no timeout during the potentially long suspend).
- **Install script**: `deploy/night-suspend/install.sh`
  - Copies all files to the system paths.
  - Seeds `/etc/default/picture-frame-night-suspend` on first install.
  - Unmasks `sleep.target` and `suspend.target` (required for `systemctl suspend`).
  - Enables and starts the timer by default (`systemctl enable --now
    picture-frame-night-suspend.timer`).
  - `--no-enable` flag leaves the timer disabled for testing.

### How to tune from settings

The new `suspend_schedule` config field is part of the regular config system:

- **Via the web UI**: open the settings page → "Suspend schedule" → adjust the
  times → save. The UI sends a `PUT /api/config` with the new `suspend_schedule`
  object. On the **next restart** of the frame, the install script (or a startup
  hook) will re‑write `/etc/default/picture-frame-night-suspend` from the config,
  and the timer will use the new times.
- **Directly edit** `runtime-overrides.toml`:
  ```toml
  [suspend_schedule]
  enabled = true
  suspend_start = "23:00"
  suspend_end = "06:30"
  wake_at = "07:00"
  ```
  After saving, restart the frame (or run the install script again) to pick up
  the change.

### Wi‑Fi resume trick (verify before enabling the timer)

The Surface 3's `mwifiex` driver can lose the Wi‑Fi connection after S2 idle
suspend. **Always test a short suspend first**:

```sh
# On the frame, as root:
sudo systemctl unmask sleep.target suspend.target
# Suspend for 60 seconds; the RTC wakes us.
sudo /usr/sbin/rtcwake -m mem -s 60
# After wake, check connectivity:
ssh surface3@100.78.238.89 'nmcli device status; systemctl is-active picture-frame kiosk-browser 2>/dev/null'
```

- **If Wi‑Fi survives**: the nightly timer is safe to enable.
- **If Wi‑Fi is dead** after wake: do **not** enable the timer yet. A post‑resume
  recovery hook can be added (restart NetworkManager, or `modprobe -r
  mwifiex_pcie mwifiex; modprobe mwifiex_pcie; systemctl restart NetworkManager`).
  See the full flow in the draft below.

### Battery note

This feature saves power overnight but does not charge the battery. The frame
still needs a working charger; at ~21% and discharging, a night suspension may
not be enough to reach morning. Fix the charger/cable/port before relying on
overnight suspend.

## Status

- Config field `suspend_schedule` added and compiles.
- Suspend script logic validated (window exit, RTC arming).
- Systemd timer/service/install script created.
- Files staged on the frame at `~/pf-staging/night-suspend/`.
- `drafts/night-suspend.md` is gitignored and contains the full idea.
- To activate on the frame: run `sudo bash ~/pf-staging/night-suspend/install.sh`,
  test a 60‑second suspend with `sudo /usr/sbin/rtcwake -m mem -s 60`, verify Wi‑Fi
  survives, then the timer will start firing at 23:00 automatically.