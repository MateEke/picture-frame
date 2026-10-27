# Surface 3 — low-power sleep and automatic wake: findings

Device: Microsoft Surface 3 (Atom x7-Z8700, Cherry Trail), BIOS 1.51116.218 (2015-03-09),
Debian 13, kernel `7.2.11-surface-1`, 1.8 GiB RAM. Evidence collected 2026-10-26 over SSH.

Legend: **F** = fact (observed on this device or read in source), **H** = hypothesis, **U** = untested.

## 1. Diagnosis of the night-suspend failure

**Conclusion: on this firmware/kernel, the RTC alarm is written to the CMOS
clock but nothing turns it into a wake event. Timed wake from s2idle does not
work. This is a platform limitation, not a script bug.**

Evidence chain:

1. **F** — 6 suspend attempts (script, raw `rtcwake`, with/without `-m disable`,
   60 s–3 min, before/after reboot). The alarm was always armed correctly
   (`/proc/driver/rtc` `alrm_time` = requested UTC time). In 5/6 the frame stayed
   asleep until the power button was pressed.
2. **F** — The RTC has **no interrupt line** (`/proc/interrupts` has no `rtc0`),
   `/proc/acpi/wakeup` is empty, and `/sys/firmware/acpi/interrupts/` does not exist.
   ACPI events are delivered through the `INT0002` virtual GPIO (IRQ 9, 13 759 events),
   the Cherry Trail hardware-reduced-ACPI mechanism.
3. **F** — `rtc_cmos.use_acpi_alarm=Y` (set automatically by `use_acpi_alarm_quirks()`
   for Intel BIOS ≥ 2015). With it, wake-up arming is `rtc_wake_on()` →
   `acpi_enable_event(ACPI_EVENT_RTC)`, which in ACPICA **returns `AE_OK` without
   doing anything when `acpi_gbl_reduced_hardware` is set** (`evxfevnt.c`). The RTC
   fixed event lives in the PM1 registers, which hardware-reduced platforms lack.
4. **F** — On resume `cmos_check_acpi_rtc_status()` calls `acpi_get_event_status()`,
   which (unlike enable/clear) has no reduced-hardware short-circuit and tries to read
   PM1 → fails → kernel logs `rtc_cmos PNP0B00:00: Could not get RTC status`. Seen once,
   at 18:55:25.
5. **F (from source) → the one "success" was not an RTC wake.** That message is only
   reachable when, at resume, the alarm has **not yet expired** (otherwise
   `cmos_check_wkalrm()` dispatches it and returns early). Resume was at 18:55:25,
   the alarm at 18:55:26. Something else woke the device ~1 s early; the wake source
   is unknown. In later tests the button wake came after expiry, so the alarm was
   dispatched instead (`PNP0B00:00` wakeup source: events=1, wakeups=0).
6. **F** — FADT flags `0x00308421`: `HW_REDUCED_ACPI=1`, `LOW_POWER_S0_IDLE=1`,
   `RTC_S4=0`. Kernel: `ACPI: PM: (supports S0 S5)`, i.e. no S3, no ACPI S4.

`rtc_cmos.use_acpi_alarm=0` would not help: without ACPI alarm *and* without an
IRQ, the driver clears `RTC_FEATURE_ALARM` and `rtcwake` would fail outright.

Issue [#2314](https://github.com/linux-surface/linux-surface/issues/2314)
(Surface 3, 7.2.11-surface-1, *spontaneous repeated self-suspend*; works on
6.19.8): **same kernel as installed, different symptom**. Not observed here
(`suspend_stats/success` = 1 this boot = our test). The rtc_cmos/ACPICA code path
above predates 7.2, so the regression is not a plausible cause of the wake failure.
But it is a risk now that `sleep.target`/`suspend.target` are unmasked.

## 1b. Can it hibernate and power back on by itself? (researched 2026-10-27)

**No, not through any firmware or kernel timer.** Evidence from the public Surface 3
ACPI dump ([linux-surface/acpidumps `surface_3`](https://github.com/linux-surface/acpidumps/tree/master/surface_3)),
whose FADT matches this frame (`Flags 00308421`):

- FADT: `Hardware Reduced = 1`, `PM1A Event Block Address = 0` (no PM1 registers, so no
  RTC_EN/RTC_STS wake bit), `RTC can wake system from S4 = 0`, `RTC_STS valid on S4 wake = 0`,
  `Remote Power-on capable = 0`.
- DSDT: the RTC (`PNP0B00`) declares only I/O ports 0x70-0x77, **no interrupt**. There is
  **no ACPI Time and Alarm Device (`ACPI000E`)**, the HW-reduced replacement for RTC wake
  ([acpi_tad ABI](https://mjmwired.net/kernel/Documentation/ABI/testing/sysfs-devices-platform-ACPI-TAD)).
  Only `_S5` is defined (no `_S3`/`_S4`); Linux hibernation here = write image + power off.
- UEFI: the `rtc-efi` platform device exists, but the driver is built only for non-x86
  (`CONFIG_RTC_DRV_EFI` absent), and on this firmware the EFI wake timer would drive the same
  CMOS alarm, which has no wake path.
- Microsoft "Wake-on-Power" (boot when power is reconnected) is SEMM-only and the
  [supported list](https://learn.microsoft.com/en-us/surface/wake-on-power-for-surface) starts
  at Surface Pro 7+/Laptop 3; the Surface 3 is not on it.

Hibernation itself (image + restore on button press) is still untested and would work only
with a manual power-button press every morning.

**Remaining hardware-only routes (untested):**
1. *Boot on charger connect.* Some tablets power on when charging starts. Test: shut down,
   unplug, plug in. If it boots, a timer plug that powers the charger at 07:00 gives
   automatic morning boot after a nightly `systemctl poweroff`/hibernate.
2. *Press the power button mechanically* (servo/solenoid on a timer). Works on any device; clunky.

Neither beats display-off (~1.0 W, instant, stays reachable) unless the goal is zero draw.

## 2. Capability matrix

| Capability | Status | Evidence | Required test |
|---|---|---|---|
| Display off (DPMS) | **Confirmed working** | `wlopm --off/--on DSI-1` as surface3; ~0.28 A vs ~1.17 A (§3) | — |
| s2idle entry/exit | **Confirmed working** | 6/6 entries, 6/6 resumes, `suspend_stats fail=0` | — |
| s2idle depth (S0i3) | **Not reached** | `pmc_atom` residency after a ~6 min s2idle this boot: S0I1/S0I2/S0I3 = 0 us. Kernel warns `pmc_atom: PCIE_PORT0 is in D0 prior to s2idle` on every suspend: the Wi-Fi PCIe port stays powered and blocks S0ix (H: caused by the linux-surface mwifiex quirk keeping ASPM off). The wiki's S0I3 numbers are from a much older kernel | power test would show the real (small) saving |
| Deep S3 | **Unsupported** | `mem_sleep` = `[s2idle]` only; ACPI supports S0 S5 only | — |
| Hibernation | Kernel-reported, untested (wake = power button only) | `disk: [shutdown]`, 1.9 GiB swap `mmcblk1p3` ≥ RAM, `resume=179:3` + initramfs `RESUME=` set; `hibernate.target` masked | manual `systemctl hibernate` after unmask (approval) |
| Power-button wake (s2idle) | **Confirmed working** | 5 manual wakes | — |
| Touchscreen wake | Unknown pending testing | — | touch during s2idle |
| USB/Type Cover wake | Kernel-reported, untested | xHCI `0000:00:14.0` wakeup=enabled | key press during s2idle |
| RTC timed wake from s2idle | **Unsupported** (high confidence) | §1 | FADT flag read to close it |
| RTC timed wake from S4/S5 | **Unsupported** (high confidence) | FADT `RTC_S4=0`, HW-reduced ACPI has no RTC wake-enable bit, no ACPI S4. Hibernation here = image + power off (S5) | optional `rtcwake -m off -s 120` to close it |
| Wi-Fi wake (WoWLAN) | Unknown, expected unsupported | PCI `01:00.0` wakeup=disabled; linux-surface mwifiex quirk disables host-sleep, which WoWLAN needs (per local guide); `iw` not installed | `iw phy phy0 wowlan show` |
| Wi-Fi after resume | **Confirmed working** | reassociated within ~4 s on all 6 resumes; mwifiex #1965 not reproduced | longer/repeated cycles |
| SSH/Tailscale while awake | **Confirmed working** | used throughout | — |
| Automatic Picture Frame recovery | Kernel-reported, untested | kiosk runs as `kiosk-backend.service` (active). `picture-frame.service` is a duplicate unit of the same binary failing on `bind :80` — harmless but noisy | check kiosk after resume |

## 3. Power measurements so far

**Measured** (`BAT0`, 5 samples over 8 s, screen on at 70 %, kiosk idle, on battery):
current 1.03–1.35 A at 3.64 V ≈ **3.8–4.9 W**. Low confidence (short window).
Battery: `charge_full` 6 691 mAh (84 % of design), 277 cycles → ~5 h at this draw.

**Measured, display on vs off** (2026-10-26 22:32–22:42, on battery, kiosk idle,
brightness 70, 30 s samples, `charge_now` deltas):

| State | Window | Charge used | Avg current | Avg power | Battery life (6 691 mAh) |
|---|---|---|---|---|---|
| Display on (DPMS on) | 271 s | 88 mAh | ~1.17 A | ~4.2 W | ~5.7 h |
| Display off (DPMS off, OS running) | 241 s | 19 mAh | ~0.28 A | ~1.0 W | ~23 h |

Instantaneous `current_now` agrees (on 1.05–1.41 A, off 0.23–0.47 A). Display off
cuts draw by ~75 %. 8 h night: ~2.3 Ah off vs ~9.4 Ah on.
Short windows: ±10 % at least; repeat for 30+ min before relying on decimals.
s2idle not measured; since S0ix is not reached (§2), its extra saving over
display-off is expected to be small (H).

Logger: `scripts/power-log.sh` (read-only; uses `charge_now` deltas, not %;
1 % ≈ 67 mAh here).

## 4. Options

| | Power | Resume | Reliability | Remote access while idle | Timed wake |
|---|---|---|---|---|---|
| A. Display off only | display share saved (to be measured) | instant | high | yes | not needed |
| B. Display off → s2idle | lowest feasible | button only | entry/exit OK; no auto-wake | **no** | **no** |
| C. Hibernate | ~0 | cold-boot + restore, button only | untested | no | **no** (expected) |
| D. Scheduled wake via hardware | as B | auto | needs extra hardware (e.g. microcontroller as USB HID pressing a key at 07:00), USB wake untested | no | yes, if USB wake works |
| E. Hybrid (A, then B/C) | best | manual for B/C | as B/C | partial | no |

**Recommendation: A (display off at night, OS running).** It is the only option
that guarantees the frame shows photos in the morning with no human. B/C are only
acceptable if a person pressing the power button each morning is acceptable.
D is the only route to real timed sleep+wake and requires hardware.

## 5. Next tests (need approval; all reversible)

1. Reinstall helper (`facp`, `dmesg`, `pmc`, `pmdebug` modes) → close §1.6, read S0ix residency.
2. 60 min display-off vs 60 min display-on power log (no root).
3. 30–60 min s2idle with power log running, button wake → s2idle draw.
4. Optional: hibernate test (unmask `hibernate.target`), `rtcwake -m off` test.

Rollback of everything done so far:

```bash
sudo systemctl disable --now picture-frame-night-suspend.timer
sudo rm /etc/systemd/system/picture-frame-night-suspend.{service,timer} \
        /usr/local/sbin/picture-frame-night-suspend /etc/default/picture-frame-night-suspend
sudo systemctl daemon-reload
sudo systemctl mask sleep.target suspend.target   # restore the guide's original state
sudo pf-suspend-test uninstall                    # removes helper + sudoers rule
```

## Sources

- linux-surface wiki, Surface 3 — https://github.com/linux-surface/linux-surface/wiki/Surface-3
  (S0ix with s2idle works since 4.20; shows S0I3 residency; no S3/hibernate/RTC info). Exact device, old kernel. Medium.
- linux-surface #2314 — https://github.com/linux-surface/linux-surface/issues/2314
  (Surface 3, 7.2.11-surface-1 self-suspend, open). Exact device and kernel. High for existence, unrelated symptom.
- rtc-cmos.c — https://github.com/torvalds/linux/blob/master/drivers/rtc/rtc-cmos.c
  (`use_acpi_alarm_quirks`, `rtc_wake_on`, `cmos_check_wkalrm`, `cmos_check_acpi_rtc_status`). High.
- ACPICA evxfevnt.c — https://github.com/torvalds/linux/blob/master/drivers/acpi/acpica/evxfevnt.c
  (fixed-event enable/clear are no-ops on reduced hardware). High.
- drivers/acpi/sleep.c `acpi_s2idle_wake` — https://github.com/torvalds/linux/blob/master/drivers/acpi/sleep.c. High.
- RTC fallback IRQ / "no legacy irqs" patches (Bay/Cherry Trail lack IRQ 8) —
  https://lkml.iu.edu/hypermail/linux/kernel/1803.2/07738.html,
  https://lkml.iu.edu/1811.2/06741.html. Similar hardware. Medium.
- Surface Laptop 3 (AMD) s2idle / rtc ACPI alarm — https://github.com/linux-surface/linux-surface/issues/1827. Different device. Low.
