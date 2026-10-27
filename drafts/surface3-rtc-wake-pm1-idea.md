# Idea: RTC wake on the Surface 3 by enabling the hidden PM1 RTC_EN bit

Status: **idea, untested** (written 2026-10-27). Background and the failed tests are in
[`surface3-power-report.md`](surface3-power-report.md).

## Summary

The firmware declares the platform *hardware-reduced ACPI*, so Linux believes there is
no PM1 event block and no RTC wake bit. But the Cherry Trail chip still has the legacy
Intel PM1 block at I/O `0x400`, and the firmware itself uses it (Linux powers off by
writing `0x405`). If we set `PM1_EN.RTC_EN` by hand and arm the CMOS alarm, the chip may
power on from S5 (off/hibernate), or possibly wake from s2idle, at the alarm time.

If it works: nightly **hibernate or poweroff + automatic boot at 07:00**, with near-zero
draw overnight. If not: nothing changes, display-off stays the solution.

## Why the normal path fails (recap)

- `rtc_cmos` arms wake via `acpi_enable_event(ACPI_EVENT_RTC)`. In ACPICA this is a
  **no-op on hardware-reduced platforms**, so the alarm is set but RTC_EN never is.
- FADT: `Hardware Reduced = 1`, `PM1A Event Block Address = 0`, `RTC can wake from S4 = 0`.
- DSDT: RTC (`PNP0B00`) has no IRQ, no `ACPI000E` Time and Alarm Device, only `_S5`.

## Evidence that the PM1 block is really there

| Source | What it shows |
|---|---|
| FADT (linux-surface acpidump, same flags `00308421` as this frame) | `Sleep Control Register` = SystemIO **0x405**, `Sleep Status Register` = SystemIO **0x401** |
| Interpretation | 0x405 = high byte of `PM1_CNT` (0x404): SLP_TYP bits 12:10 → byte bits 4:2, SLP_EN bit 13 → bit 5, exactly the HW-reduced sleep-control format. 0x401 = high byte of `PM1_STS` (0x400): WAK_STS bit 15 → bit 7. So **ACPI base = 0x400** and Linux already uses this block to power off |
| DSDT | `Name (PMBS, 0x0400)`; `OperationRegion (PMIO, SystemIO, PMBS, 0x46)` with `PWBS` at offset 0x01 bit 0 (= PM1_STS bit 8, PWRBTN_STS); motherboard resource reserves I/O `0x400`–`0x47F` |
| DSDT `_S5` | `Package {0x07, …}`: SLP_TYP 7 = S5, the standard Intel value |
| DSDT `_PTS` → `PPTS` | Touches only GPE status/enable fields (`G1S`, `G1E`, `G0S`), the POST code and TPM. **Does not clear PM1_EN** |
| coreboot Braswell (same SoC family) `src/soc/intel/braswell/include/soc/pm.h` | `PM1_STS 0x00`, `RTC_STS (1<<10)`, `PM1_EN 0x02`, `RTC_EN (1<<10)`, `PWRBTN_EN (1<<8)`, `PM1_CNT 0x04`; LPC `ABASE` at PCI config `0x40` of `00:1f.0` |
| Frame | `iTCO_wdt` and `lpc_ich` loaded (they locate TCO at ACPI base + 0x60, another cross-check) |

## Register map (ACPI base 0x400)

| Port | Width | Register | Bits that matter |
|---|---|---|---|
| 0x400 | 16 | PM1_STS (write 1 to clear) | 10 RTC_STS, 8 PWRBTN_STS, 15 WAK_STS |
| 0x402 | 16 | PM1_EN | **10 RTC_EN**, 8 PWRBTN_EN |
| 0x404 | 32 | PM1_CNT | 12:10 SLP_TYP, 13 SLP_EN (Linux writes the 0x405 byte) |

Verify against the Intel Atom Z8000 / Braswell datasheet before writing anything.

## Test plan

All steps need root. Run through a helper mode (like `pf-suspend-test`) so one sudoers
rule covers it. **Read-only first.**

### 0. Pre-checks (read-only)

```bash
cat /sys/kernel/security/lockdown        # [integrity] or [confidentiality] blocks /dev/port
ls -l /dev/port
setpci -s 00:1f.0 40.l                   # ABASE; expect 0x400 | enable bits (pciutils)
grep -i -E "0400|iTCO|ACPI" /proc/ioports
```

If lockdown is active (Secure Boot), `/dev/port` is refused. Options then: disable
Secure Boot in Surface UEFI, or a tiny kernel module (also blocked unless signed).

Read the registers:

```python
# /usr/local/sbin/pm1 (run as root)
import os, sys, struct
fd = os.open("/dev/port", os.O_RDWR)
def r16(p): return struct.unpack("<H", os.pread(fd, 2, p))[0]
def w16(p, v): os.pwrite(fd, struct.pack("<H", v), p)
sts, en = r16(0x400), r16(0x402)
print(f"PM1_STS=0x{sts:04x} RTC_STS={sts>>10&1}  PM1_EN=0x{en:04x} RTC_EN={en>>10&1} PWRBTN_EN={en>>8&1}")
if sys.argv[1:] == ["arm"]:
    w16(0x400, 1 << 10)            # clear stale RTC_STS only (W1C: never write other bits)
    w16(0x402, en | (1 << 10))     # read-modify-write: add RTC_EN
    print(f"PM1_EN now 0x{r16(0x402):04x}")
```

Sanity: with the CMOS alarm armed and expired, `RTC_STS` should read 1 (shows the alarm
logic feeds this block at all).

### 1. S5 test (cleanest, covers hibernate too)

```bash
rtcwake -d /dev/rtc0 -m no -s 180     # alarm in 3 min, does not sleep
pm1 arm                                # set RTC_EN
systemctl poweroff
```

Expected if it works: the tablet powers on by itself ~3 min later. If not: press power.
Hibernation is image + the same S5 power-off, so S5 success ≈ hibernate success.

Better placement once it works: set RTC_EN as late as possible, in a
`/usr/lib/systemd/system-shutdown/` hook (runs right before the final poweroff), in case
something on the way down clears it.

### 2. Hibernate test (only if 1 works)

Needs approval: `systemctl unmask hibernate.target`; swap `mmcblk1p3` (1.9 GiB) and
`resume=` / initramfs `RESUME=` are already configured. Same arming, then
`systemctl hibernate`. Check the kiosk, Wi-Fi and Tailscale after restore.

### 3. s2idle test (least likely)

Same arming, then `systemctl suspend`. Waking from S0ix needs the RTC event to reach an
interrupt (SCI, IRQ 9, shared with the `INT0002` virtual GPIO). With
`pm_debug_messages=1`, the log shows whether anything arrived. Expect failure.

## Risks and rollback

- Register writes are volatile: a power cycle or reboot resets them. Rollback =
  `w16(0x402, en & ~(1<<10))`, or just reboot.
- PM1_STS is write-1-to-clear: write **only** bit 10, or you clear other status bits.
- Possible spurious wake or instant power-on after poweroff if RTC_STS was not cleared;
  the power button still works.
- Worst realistic case: the frame stays off until the button is pressed (same as today).
- Do not touch PM1_CNT or other bits by hand; Linux owns the sleep transition.

## If it works: integration

1. Night: a timer at 23:00 runs `rtcwake -m no -t <07:00 epoch>` and the arm step, then
   `systemctl hibernate` (or poweroff).
2. Morning: the chip powers on at 07:00, resumes or boots, and the kiosk starts as at boot.
3. Keep the display-off schedule as the fallback; never enable the timer until 3+
   consecutive automatic wakes have been observed.
4. Measure overnight drain with `scripts/power-log.sh` (charge_now before/after).

## Open questions

- Does the Surface firmware or EC cut the RTC power-on path in S5 regardless of RTC_EN?
- Does Linux (ACPICA, hardware-reduced) or `_PTS` write PM1_EN on the way down? `_PTS` doesn't
  per the DSDT; ACPICA's reduced-hardware sleep path writes only the sleep control register.
- Does the Surface 3 boot automatically when the charger is connected? (Separate fallback:
  a timer plug on the charger.)

## References

- linux-surface ACPI dumps, Surface 3 (FADT/DSDT): https://github.com/linux-surface/acpidumps/tree/master/surface_3
- coreboot Braswell PM registers: https://github.com/coreboot/coreboot/blob/main/src/soc/intel/braswell/include/soc/pm.h
- coreboot Braswell LPC (`ABASE 0x40`): https://github.com/coreboot/coreboot/blob/main/src/soc/intel/braswell/include/soc/lpc.h
- rtc-cmos wake path: https://github.com/torvalds/linux/blob/master/drivers/rtc/rtc-cmos.c
- ACPICA fixed events, reduced-hardware no-ops: https://github.com/torvalds/linux/blob/master/drivers/acpi/acpica/evxfevnt.c
