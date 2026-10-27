#!/usr/bin/env bash
# Read-only battery logger for measuring power draw in different states
# (display on, display off, s2idle, hibernate). Needs no root.
#
#   scripts/power-log.sh [interval-seconds] >> power.csv
#
# Each row is one sample. Keep it running across a suspend: the first row
# after resume has a larger wall-clock gap and a higher suspend_success count,
# and the charge_uah delta across that gap is the average draw while asleep.
# Compare charge_uah deltas over long windows (30+ min); capacity_pct is
# rounded to 1% (~67 mAh on this battery) and is useless for short tests.
set -euo pipefail

interval=${1:-60}
bat=${BAT:-/sys/class/power_supply/BAT0}
bl=$(ls -d /sys/class/backlight/* 2>/dev/null | head -1)

read_or() { cat "$1" 2>/dev/null || echo "$2"; }

echo "epoch,time,status,capacity_pct,voltage_uv,current_ua,charge_uah,power_mw,brightness,bl_power,suspend_success,load1"
while :; do
    v=$(read_or "$bat/voltage_now" 0)
    i=$(read_or "$bat/current_now" 0)
    printf '%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n' \
        "$(date +%s)" "$(date +%FT%T%z)" \
        "$(read_or "$bat/status" ?)" "$(read_or "$bat/capacity" ?)" \
        "$v" "$i" "$(read_or "$bat/charge_now" ?)" \
        "$((v / 1000 * (i / 1000) / 1000))" \
        "$(read_or "$bl/brightness" ?)" "$(read_or "$bl/bl_power" ?)" \
        "$(read_or /sys/power/suspend_stats/success ?)" \
        "$(cut -d' ' -f1 /proc/loadavg)"
    sleep "$interval"
done
