#!/usr/bin/env bash
# Fetch, verify, and provision the picture-frame on a Raspberry Pi. See --help.
set -euo pipefail

REPO="MateEke/picture-frame"
# Trust anchor; must match deploy/minisign.pub and the updater's embedded key.
MINISIGN_PUBKEY="RWSn8v6e9fllWNQOZr6w2z8ic5NXGHtKMWxjxrjuu3SWj8BpoWxQLtHl"

DRY_RUN=0
NON_INTERACTIVE=0
DO_UNINSTALL=0
VERSION="latest"
ARCH_OVERRIDE=""
INSTALL_DIR=""
SERVICE_USER=""
DISPLAY_BACKEND="wlopm"
DISPLAY_BACKEND_SET=0
DISPLAY_OUTPUT=""
DISPLAY_OUTPUT_SET=0
GITHUB_TOKEN=""
NO_AP=0
AP_SSID="PictureFrame"
AP_SSID_SET=0
AP_PASSWORD=""
AP_PASSWORD_SET=0
APP_PASSWORD=""
APP_PASSWORD_SET=0
UNATTENDED_UPGRADES=1
UNATTENDED_UPGRADES_SET=0

ARCH=""
WLAN_IFACE=""
HOSTNAME=""
HDMI_CONN=""
SRC_DIR=""
APT_UPDATED=0
RELEASE_TAG=""
RELEASE_JSON=""
TARBALL=""
WORKDIR=""

log()  { printf '==> %s\n' "$*"; }
warn() { printf 'WARN: %s\n' "$*" >&2; }
err()  { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

# return 0 so an empty WORKDIR (dry-run) doesn't make the trap's status exit 1.
cleanup() { [ -n "${WORKDIR:-}" ] && rm -rf "$WORKDIR"; return 0; }
trap cleanup EXIT

run_cmd() {
    if [ "$DRY_RUN" -eq 1 ]; then
        printf '[dry-run] %s\n' "$*"
    else
        "$@"
    fi
}

write_file() {
    local path="$1"
    if [ "$DRY_RUN" -eq 1 ]; then
        printf '[dry-run] write %s:\n' "$path"
        sed 's/^/    | /'
    else
        cat > "$path"
    fi
}

# apt-get update runs once per invocation.
apt_install() {
    if [ "$APT_UPDATED" -eq 0 ]; then
        run_cmd apt-get update
        APT_UPDATED=1
    fi
    run_cmd apt-get install -y "$@"
}

remove_unit() {
    local name="$1" path="/etc/systemd/system/$1.service"
    if [ -f "$path" ] || [ "$DRY_RUN" -eq 1 ]; then
        run_cmd systemctl disable --now "$name.service" 2>/dev/null || true
        run_cmd rm -f "$path"
    fi
}

# /dev/tty so prompts work under `curl | sudo bash` (stdin is the script).
prompt()        { read -r -p "$1" "${2?}" < /dev/tty; }
prompt_secret() { read -r -s -p "$1" "${2?}" < /dev/tty; printf '\n' > /dev/tty; }

# Echoes the path or "". Must return 0: a non-zero return aborts `cfg="$(...)"` under set -e.
find_boot_file() {
    local f
    for f in "/boot/firmware/$1" "/boot/$1"; do
        [ -f "$f" ] && { printf '%s' "$f"; return 0; }
    done
    return 0
}

gh_curl() {
    if [ -n "$GITHUB_TOKEN" ]; then
        curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" "$@"
    else
        curl -fsSL "$@"
    fi
}

ap_wanted() { [ "$NO_AP" -eq 0 ] && [ "$AP_SSID_SET" -eq 1 ]; }

# Sets `key = value` under [section], replacing a commented/existing key, else
# appending. value is verbatim (caller quotes). via ENVIRON, not -v: -v eats backslashes.
set_toml_key() {
    local file="$1" section="[$2]" key="$3" value="$4" tmp
    tmp="$(mktemp)"
    pf_val="$value" awk -v section="$section" -v key="$key" '
        BEGIN { in_section=0; done=0; value=ENVIRON["pf_val"] }
        /^\[/ {
            if (in_section && !done) { print key " = " value; done=1 }
            in_section = ($0 == section)
            print; next
        }
        {
            if (in_section && !done) {
                line=$0; sub(/^[[:space:]]*#?[[:space:]]*/, "", line)
                if (line ~ "^" key "[[:space:]]*=") { print key " = " value; done=1; next }
            }
            print
        }
        END {
            if (in_section && !done) { print key " = " value; done=1 }
            if (!done) { print ""; print section; print key " = " value }
        }
    ' "$file" > "$tmp"
    write_file "$file" < "$tmp"
    rm -f "$tmp"
}

usage() {
    cat <<'EOF'
install.sh: fetch, verify, and provision the picture-frame.

Usage:
  curl -fsSL https://github.com/MateEke/picture-frame/releases/latest/download/install.sh | sudo bash
  sudo bash install.sh [flags]
  sudo bash install.sh --uninstall

Flags:
  --version <tag>             Install a specific release (default: latest stable).
  --arch <armv6|armv7|arm64>  Override CPU-arch autodetection.
  --install-dir <path>        Install location (default: ~<user>/picture-frame).
  --user <name>               Service user (default: the invoking sudo user).
  --display-backend <name>    wlopm (default, recommended) or vcgencmd.
                              vcgencmd is a legacy fallback: lighter on RAM/CPU
                              (no compositor) but showed instability in testing
                              that labwc/wlopm does not.
  --display-output <name>     Override detected HDMI connector (e.g. HDMI-A-2).
  --ssid <name>               Enable the WiFi-recovery AP with this SSID.
  --ap-password <pw>          AP password (optional; open AP if omitted).
  --no-ap                     Do not configure the AP fallback.
  --app-password <pw>         Admin web-UI password (bcrypt-hashed locally).
  --github-token <token>      Token for fetching from a private repo.
  --no-unattended-upgrades    Skip enabling automatic OS security updates
                              (enabled by default; security origin only, no reboot).
  --non-interactive, --yes    Never prompt; use flags/defaults only.
  --dry-run                   Print actions without executing them.
  --uninstall                 Reverse a previous install and exit.
  -h, --help                  Show this help.
EOF
}

parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
            --version)         VERSION="${2:?--version requires a value}"; shift 2 ;;
            --arch)            ARCH_OVERRIDE="${2:?--arch requires a value}"; shift 2 ;;
            --install-dir)     INSTALL_DIR="${2:?--install-dir requires a value}"; shift 2 ;;
            --user)            SERVICE_USER="${2:?--user requires a value}"; shift 2 ;;
            --display-backend) DISPLAY_BACKEND="${2:?--display-backend requires a value}"; DISPLAY_BACKEND_SET=1; shift 2 ;;
            --display-output)  DISPLAY_OUTPUT="${2:?--display-output requires a value}"; DISPLAY_OUTPUT_SET=1; shift 2 ;;
            --ssid)            AP_SSID="${2:?--ssid requires a value}"; AP_SSID_SET=1; shift 2 ;;
            --ap-password)     AP_PASSWORD="${2:?--ap-password requires a value}"; AP_PASSWORD_SET=1; shift 2 ;;
            --no-ap)           NO_AP=1; shift ;;
            --app-password)    APP_PASSWORD="${2:?--app-password requires a value}"; APP_PASSWORD_SET=1; shift 2 ;;
            --github-token)    GITHUB_TOKEN="${2:?--github-token requires a value}"; shift 2 ;;
            --no-unattended-upgrades) UNATTENDED_UPGRADES=0; UNATTENDED_UPGRADES_SET=1; shift ;;
            --non-interactive|--yes) NON_INTERACTIVE=1; shift ;;
            --dry-run)         DRY_RUN=1; shift ;;
            --uninstall)       DO_UNINSTALL=1; shift ;;
            -h|--help)         usage; exit 0 ;;
            *) err "unknown argument: $1 (see --help)" ;;
        esac
    done
    case "$DISPLAY_BACKEND" in
        wlopm|vcgencmd) ;;
        *) err "invalid --display-backend: $DISPLAY_BACKEND (want wlopm or vcgencmd)" ;;
    esac
}

require_root() {
    [ "$DRY_RUN" -eq 1 ] && return
    [ "$(id -u)" -eq 0 ] || err "must run as root, re-run with: sudo bash install.sh"
}

require_tty() {
    [ "$NON_INTERACTIVE" -eq 1 ] && return
    if [ ! -e /dev/tty ] || ! { : < /dev/tty; } 2>/dev/null; then
        err "no terminal for prompts, re-run with --non-interactive (and flags)"
    fi
}

resolve_user_and_dir() {
    [ -n "$SERVICE_USER" ] || SERVICE_USER="${SUDO_USER:-$(id -un)}"
    if [ -z "$INSTALL_DIR" ]; then
        local home
        home="$(getent passwd "$SERVICE_USER" | cut -d: -f6)"
        INSTALL_DIR="${home:-/home/$SERVICE_USER}/picture-frame"
    fi
}

detect_arch() {
    if [ -n "$ARCH_OVERRIDE" ]; then ARCH="$ARCH_OVERRIDE"; return; fi
    local m; m="$(uname -m)"
    case "$m" in
        aarch64) ARCH="arm64" ;;
        armv7l)  ARCH="armv7" ;;
        armv6l)  ARCH="armv6" ;;
        *)
            # --dry-run must run on a non-Pi dev box; assume a target rather than abort.
            if [ "$DRY_RUN" -eq 1 ]; then
                warn "non-Pi arch '$m'; dry-run assuming arm64 (use --arch to override)"
                ARCH="arm64"
            else
                err "unsupported architecture '$m'; pass --arch armv6|armv7|arm64"
            fi
            ;;
    esac
}

resolve_release() {
    local api
    if [ "$VERSION" = "latest" ]; then
        api="https://api.github.com/repos/$REPO/releases/latest"
    else
        api="https://api.github.com/repos/$REPO/releases/tags/$VERSION"
    fi
    RELEASE_JSON="$(gh_curl "$api")" || err "could not query the release from $REPO"
    if [ "$VERSION" = "latest" ]; then
        # || true: an absent tag_name fails the pipeline, aborting before the check under set -e.
        RELEASE_TAG="$(printf '%s' "$RELEASE_JSON" | grep -m1 '"tag_name"' \
            | sed -E 's/.*"tag_name":[[:space:]]*"([^"]+)".*/\1/' || true)"
        [ -n "$RELEASE_TAG" ] || err "could not parse the latest release tag from $REPO (rate-limited? try --version)"
    else
        RELEASE_TAG="$VERSION"
    fi
    # goreleaser strips the leading v: v1.2.3 -> picture-frame_1.2.3_...
    TARBALL="picture-frame_${RELEASE_TAG#v}_linux_${ARCH}.tar.gz"
    log "release $RELEASE_TAG ($ARCH) -> $TARBALL"
}

# Numeric asset id for a name in RELEASE_JSON. "id" sits 2 lines above "name".
asset_id_for() {
    printf '%s' "$RELEASE_JSON" | grep -B3 -F "\"name\": \"$1\"" \
        | grep -oE '"id": [0-9]+' | head -1 | grep -oE '[0-9]+'
}

# API asset endpoint, not browser_download_url (which 404s on private repos even with a token).
download_asset() {
    local id
    id="$(asset_id_for "$1")"
    [ -n "$id" ] || err "release $RELEASE_TAG has no asset named $1"
    gh_curl -H "Accept: application/octet-stream" \
        "https://api.github.com/repos/$REPO/releases/assets/$id" -o "$2" \
        || err "download of $1 failed"
}

download_release() {
    WORKDIR="$(mktemp -d)"
    log "downloading release assets"
    download_asset "$TARBALL"              "$WORKDIR/$TARBALL"
    download_asset "checksums.txt"         "$WORKDIR/checksums.txt"
    download_asset "checksums.txt.minisig" "$WORKDIR/checksums.txt.minisig"
}

verify_release() {
    log "verifying signature and checksum"
    command -v minisign >/dev/null || err "minisign missing (the dependency step should have installed it)"
    ( cd "$WORKDIR" && minisign -V -P "$MINISIGN_PUBKEY" -m checksums.txt -x checksums.txt.minisig ) \
        || err "minisign signature verification failed"
    ( cd "$WORKDIR" && grep " ${TARBALL}\$" checksums.txt | sha256sum -c - ) \
        || err "sha256 checksum mismatch for $TARBALL"
}

extract_release() {
    log "installing to $INSTALL_DIR"
    run_cmd mkdir -p "$INSTALL_DIR"
    # Unwrapped: unreachable under --dry-run (fetch_and_verify returns early).
    tar -xzf "$WORKDIR/$TARBALL" -C "$INSTALL_DIR"
    run_cmd chown -R "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR"
}

fetch_and_verify() {
    if [ -x "$INSTALL_DIR/picture-frame" ] && [ "$VERSION" = "latest" ]; then
        log "existing binary at $INSTALL_DIR/picture-frame, skipping download (use --version to force)"
        return
    fi
    if [ "$DRY_RUN" -eq 1 ]; then
        log "[dry-run] would resolve/download/verify/extract release '$VERSION' for $ARCH"
        return
    fi
    resolve_release
    download_release
    verify_release
    extract_release
}

# deploy/ templates: beside this script when run as a file, else the extracted tree.
determine_src_dir() {
    local self="${BASH_SOURCE[0]:-}"
    if [ -n "$self" ] && [ -f "$self" ]; then
        local d; d="$(cd "$(dirname "$self")" && pwd)"
        if [ -d "$d/systemd" ]; then SRC_DIR="$d"; return; fi
    fi
    SRC_DIR="$INSTALL_DIR/deploy"
}

install_runtime_deps() {
    # seatd brokers seat/DRM access for the User= compositor/browser (no login session).
    local pkgs="seatd cog dnsmasq-base bluez avahi-daemon"
    [ "$DISPLAY_BACKEND" = "wlopm" ] && pkgs="$pkgs labwc wlopm"
    log "installing runtime packages: $pkgs"
    # shellcheck disable=SC2086 # intentional word-splitting of the package list
    apt_install $pkgs
}

# Fresh Pi OS soft-blocks Bluetooth via rfkill; unblock so the (optional) BLE
# sensor adapter can power on. systemd-rfkill persists this across reboots.
enable_bluetooth() {
    if command -v rfkill >/dev/null; then
        run_cmd rfkill unblock bluetooth
    else
        warn "rfkill not found; for BLE sensors run: sudo rfkill unblock bluetooth"
    fi
}

# Debian's default config is security-origin-only, so we add nothing, debconf
# just turns the periodic run on. No reboot policy: kernel fixes wait for the next
# power cycle (a forced Zero reboot flashes the screen for minutes).
configure_unattended_upgrades() {
    if [ "$UNATTENDED_UPGRADES" -eq 0 ]; then
        log "automatic OS security updates: disabled"
        return
    fi
    log "enabling automatic OS security updates (unattended-upgrades)"
    # Preseed enable=true before install so the package's setup writes an enabled
    # 20auto-upgrades regardless of the image's default.
    run_cmd debconf-set-selections <<<'unattended-upgrades unattended-upgrades/enable_auto_updates boolean true'
    apt_install unattended-upgrades
}

check_networkmanager() {
    if [ "$DRY_RUN" -eq 1 ]; then
        log "[dry-run] would verify NetworkManager is the active network backend"
        return
    fi
    if ! systemctl is-active --quiet NetworkManager; then
        err "NetworkManager is not active. This installer needs NM (Raspberry Pi OS default). Enable it (raspi-config -> Advanced -> Network Config -> NetworkManager) and re-run."
    fi
}

set_hostname() {
    WLAN_IFACE=""
    local dev
    for dev in /sys/class/net/*; do
        if [ -e "$dev/wireless" ] || [ -e "$dev/phy80211" ]; then
            WLAN_IFACE="$(basename "$dev")"; break
        fi
    done
    if [ -z "$WLAN_IFACE" ] || [ ! -r "/sys/class/net/$WLAN_IFACE/address" ]; then
        if [ "$DRY_RUN" -eq 1 ]; then
            log "[dry-run] no wireless iface on this host; on the Pi the hostname derives from its MAC"
            return
        fi
        err "no wireless interface found; cannot derive a unique hostname. Enable WiFi and re-run."
    fi
    local suffix; suffix="$(awk -F: '{printf "%s%s", $5, $6}' "/sys/class/net/$WLAN_IFACE/address")"
    HOSTNAME="pictureframe-${suffix}"
    log "setting hostname to $HOSTNAME (from $WLAN_IFACE)"
    run_cmd hostnamectl set-hostname "$HOSTNAME"
}

install_polkit() {
    log "installing polkit rule"
    local dst="/etc/polkit-1/rules.d/50-pictureframe-networkmanager.rules"
    sed "s/@@USER@@/$SERVICE_USER/g" "$SRC_DIR/polkit/50-pictureframe-networkmanager.rules" | write_file "$dst"
    run_cmd chmod 644 "$dst"
}

install_units() {
    log "installing systemd units ($DISPLAY_BACKEND)"
    local units
    if [ "$DISPLAY_BACKEND" = "wlopm" ]; then
        units="labwc kiosk-browser kiosk-backend picture-frame-rollback"
        remove_unit kiosk-browser-drm
    else
        units="kiosk-browser-drm kiosk-backend picture-frame-rollback"
        remove_unit labwc
        remove_unit kiosk-browser
    fi
    local svc src dst
    for svc in $units; do
        src="$SRC_DIR/systemd/${svc}.service"
        [ -f "$src" ] || { warn "missing unit template $src"; continue; }
        dst="/etc/systemd/system/${svc}.service"
        sed -e "s|@@USER@@|$SERVICE_USER|g" -e "s|@@INSTALL_DIR@@|$INSTALL_DIR|g" "$src" | write_file "$dst"
        run_cmd chmod 644 "$dst"
    done
    if [ "$DISPLAY_BACKEND" = "wlopm" ]; then
        log "installing kiosk launch script"
        run_cmd install -m 0755 "$SRC_DIR/kiosk-browser-launch.sh" "$INSTALL_DIR/kiosk-browser-launch.sh"
    fi
}

install_labwc() {
    [ "$DISPLAY_BACKEND" = "wlopm" ] || return 0
    log "installing labwc config"
    run_cmd mkdir -p "$INSTALL_DIR/labwc"
    if [ -d "$SRC_DIR/labwc" ]; then
        run_cmd cp -r "$SRC_DIR/labwc/." "$INSTALL_DIR/labwc/"
        run_cmd chown -R "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR/labwc"
    fi
}

detect_hdmi_connector() {
    if [ -n "$DISPLAY_OUTPUT" ]; then HDMI_CONN="$DISPLAY_OUTPUT"; return; fi
    HDMI_CONN=""
    local st
    for st in /sys/class/drm/card*-HDMI-A-*/status; do
        [ -e "$st" ] || continue
        if [ "$(cat "$st")" = "connected" ]; then
            HDMI_CONN="$(basename "$(dirname "$st")")"
            HDMI_CONN="${HDMI_CONN#card*-}"
            break
        fi
    done
    HDMI_CONN="${HDMI_CONN:-HDMI-A-1}"
}

configure_display() {
    set_display_overlay
    if [ "$DISPLAY_BACKEND" = "wlopm" ]; then
        pin_hdmi_cmdline
    else
        log "vcgencmd backend: skipping HDMI cmdline pin (full-KMS-only fix)"
    fi
}

set_display_overlay() {
    local cfg want
    cfg="$(find_boot_file config.txt)"
    if [ "$DISPLAY_BACKEND" = "wlopm" ]; then want="vc4-kms-v3d"; else want="vc4-fkms-v3d"; fi
    if [ -z "$cfg" ]; then
        warn "no config.txt found; ensure 'dtoverlay=$want' is set manually"
        return
    fi
    log "setting dtoverlay=$want in $cfg"
    [ -f "$cfg.pictureframe.bak" ] || run_cmd cp "$cfg" "$cfg.pictureframe.bak"
    local tmp; tmp="$(mktemp)"
    awk -v want="$want" -v backend="$DISPLAY_BACKEND" '
        BEGIN { seen_overlay=0; seen_gpu=0 }
        /^[[:space:]]*dtoverlay=vc4-(kms|fkms)-v3d/ {
            if ($0 ~ "dtoverlay=" want) { print "dtoverlay=" want; seen_overlay=1 }
            else { print "#" $0 }
            next
        }
        /^[[:space:]]*#?[[:space:]]*gpu_mem=/ {
            if (backend == "vcgencmd") { print "gpu_mem=128"; seen_gpu=1; next }
            print; next
        }
        { print }
        END {
            if (!seen_overlay) print "dtoverlay=" want
            if (backend == "vcgencmd" && !seen_gpu) print "gpu_mem=128"
        }
    ' "$cfg" > "$tmp"
    write_file "$cfg" < "$tmp"
    rm -f "$tmp"
}

# Forces the connector connected so a DPMS-off panel's HPD drop doesn't lose the output.
pin_hdmi_cmdline() {
    local cmdline; cmdline="$(find_boot_file cmdline.txt)"
    if [ -z "$cmdline" ]; then
        warn "no cmdline.txt found; skipping HDMI pin (screen power-off may be unreliable)"
        return
    fi
    local line orig; line="$(cat "$cmdline")"; orig="$line"
    case " $line " in
        *" video=$HDMI_CONN:"*|*" video=$HDMI_CONN "*) ;;
        *) line="$line video=$HDMI_CONN:e" ;;
    esac
    case " $line " in
        *" drm_kms_helper.poll="*) ;;
        *) line="$line drm_kms_helper.poll=0" ;;
    esac
    if [ "$line" != "$orig" ]; then
        [ -f "$cmdline.pictureframe.bak" ] || run_cmd cp "$cmdline" "$cmdline.pictureframe.bak"
        printf '%s\n' "$line" | write_file "$cmdline"
        log "pinned HDMI ($HDMI_CONN) in $cmdline, reboot required"
    else
        log "$cmdline already pinned ($HDMI_CONN)"
    fi
}

gather_input() {
    [ "$NON_INTERACTIVE" -eq 1 ] && return
    if [ "$NO_AP" -eq 0 ] && [ "$AP_SSID_SET" -eq 0 ]; then
        local ans ssid_in
        prompt "Configure the WiFi-recovery access point? [y/N] " ans
        case "$ans" in
            [Yy]*)
                prompt "  AP SSID [$AP_SSID]: " ssid_in
                [ -n "${ssid_in:-}" ] && AP_SSID="$ssid_in"
                AP_SSID_SET=1
                prompt_secret "  AP password (recommended; blank = open network): " AP_PASSWORD
                [ -n "$AP_PASSWORD" ] && AP_PASSWORD_SET=1
                ;;
            *) NO_AP=1 ;;
        esac
    fi
    if [ "$APP_PASSWORD_SET" -eq 0 ]; then
        local p1 p2
        prompt_secret "Set an admin web-UI password (recommended; blank = no password): " p1
        if [ -n "$p1" ]; then
            prompt_secret "  confirm: " p2
            [ "$p1" = "$p2" ] || err "passwords did not match"
            APP_PASSWORD="$p1"; APP_PASSWORD_SET=1
        fi
    fi
    if [ "$UNATTENDED_UPGRADES_SET" -eq 0 ]; then
        local uu
        prompt "Enable automatic OS security updates (recommended)? [Y/n] " uu
        case "$uu" in [Nn]*) UNATTENDED_UPGRADES=0 ;; esac
    fi
}

seed_config() {
    local cfg="$INSTALL_DIR/config.toml" fresh=0
    [ -f "$cfg" ] || fresh=1

    if [ "$DRY_RUN" -eq 1 ]; then
        log "[dry-run] would seed $cfg (fresh=$fresh): display.backend=$DISPLAY_BACKEND display.output=$HDMI_CONN wifi.ap_ssid=$AP_SSID app_password=$([ "$APP_PASSWORD_SET" -eq 1 ] && echo set || echo unchanged)"
        return
    fi

    if [ "$fresh" -eq 1 ]; then
        log "seeding new $cfg from config.example.toml"
        run_cmd cp "$INSTALL_DIR/config.example.toml" "$cfg"
        run_cmd chown "$SERVICE_USER:$SERVICE_USER" "$cfg"
        run_cmd chmod 600 "$cfg"
    else
        log "existing $cfg left intact; updating only explicitly-flagged fields"
    fi

    if [ "$fresh" -eq 1 ] || [ "$DISPLAY_BACKEND_SET" -eq 1 ]; then
        set_toml_key "$cfg" display backend "\"$DISPLAY_BACKEND\""
    fi
    if [ "$fresh" -eq 1 ] || [ "$DISPLAY_OUTPUT_SET" -eq 1 ]; then
        set_toml_key "$cfg" display output "\"$HDMI_CONN\""
    fi
    if [ "$fresh" -eq 1 ] || [ "$AP_SSID_SET" -eq 1 ]; then
        set_toml_key "$cfg" wifi ap_ssid "\"$AP_SSID\""
    fi
    if [ "$fresh" -eq 1 ] || [ "$AP_PASSWORD_SET" -eq 1 ]; then
        set_toml_key "$cfg" wifi ap_password "\"$AP_PASSWORD\""
    fi
    if [ "$APP_PASSWORD_SET" -eq 1 ]; then
        local hash
        hash="$(printf '%s' "$APP_PASSWORD" | "$INSTALL_DIR/picture-frame" -hash-password)" \
            || err "failed to hash the admin password"
        set_toml_key "$cfg" auth password_hash "\"$hash\""
    fi
}

configure_ap() {
    if ! ap_wanted; then
        log "AP fallback disabled"
        run_cmd nmcli connection delete hotspot 2>/dev/null || true
        return
    fi
    log "creating hotspot AP profile (SSID: $AP_SSID)"
    run_cmd nmcli connection delete hotspot 2>/dev/null || true
    local sec=()
    # psk briefly in argv (root install window). pmf=1: brcmfmac AP can't do 802.11w.
    [ "$AP_PASSWORD_SET" -eq 1 ] && sec=(wifi-sec.key-mgmt wpa-psk wifi-sec.psk "$AP_PASSWORD" wifi-sec.pmf 1)
    # band bg: the Pi radio is 2.4GHz-only. "${sec[@]+...}" guards the empty array (set -u, bash <4.4).
    run_cmd nmcli connection add type wifi ifname "$WLAN_IFACE" con-name hotspot \
        autoconnect no wifi.mode ap wifi.ssid "$AP_SSID" wifi.band bg \
        ipv4.method shared ipv4.addresses 192.168.42.1/24 ipv6.method disabled "${sec[@]+"${sec[@]}"}"
    log "installing dnsmasq captive-portal config"
    run_cmd mkdir -p /etc/NetworkManager/dnsmasq-shared.d
    run_cmd install -m 0644 "$SRC_DIR/dnsmasq.d/captive-portal.conf" \
        /etc/NetworkManager/dnsmasq-shared.d/captive-portal.conf
}

enable_and_finish() {
    log "enabling services"
    run_cmd systemctl daemon-reload
    if [ "$DISPLAY_BACKEND" = "wlopm" ]; then
        run_cmd systemctl enable seatd.service labwc.service kiosk-backend.service kiosk-browser.service
    else
        run_cmd systemctl enable seatd.service kiosk-backend.service kiosk-browser-drm.service
    fi
    printf '\n'
    log "done, reachable at ${HOSTNAME:-<device>}.local after reboot"
    if [ "$NON_INTERACTIVE" -eq 1 ] || [ "$DRY_RUN" -eq 1 ]; then
        log "reboot to apply: sudo reboot"
        return
    fi
    local ans; prompt "Reboot now? [y/N] " ans
    case "$ans" in [Yy]*) run_cmd reboot ;; *) log "reboot later with: sudo reboot" ;; esac
}

uninstall() {
    log "uninstalling picture-frame"
    local svc
    for svc in labwc kiosk-browser kiosk-browser-drm kiosk-backend picture-frame-rollback; do
        remove_unit "$svc"
    done
    run_cmd systemctl daemon-reload
    run_cmd nmcli connection delete hotspot 2>/dev/null || true
    run_cmd rm -f /etc/NetworkManager/dnsmasq-shared.d/captive-portal.conf
    run_cmd rm -f /etc/polkit-1/rules.d/50-pictureframe-networkmanager.rules
    local f
    for f in /boot/firmware/cmdline.txt /boot/cmdline.txt /boot/firmware/config.txt /boot/config.txt; do
        if [ -f "$f.pictureframe.bak" ]; then
            run_cmd cp "$f.pictureframe.bak" "$f"
            run_cmd rm -f "$f.pictureframe.bak"
            log "restored $f"
        fi
    done
    log "left install dir, config, and images in place: $INSTALL_DIR"
    log "hostname not reverted (set manually: sudo hostnamectl set-hostname <name>)"
}

main() {
    parse_args "$@"
    require_root
    resolve_user_and_dir
    if [ "$DO_UNINSTALL" -eq 1 ]; then uninstall; exit 0; fi
    require_tty
    detect_arch
    gather_input
    log "installer: user=$SERVICE_USER dir=$INSTALL_DIR backend=$DISPLAY_BACKEND arch=$ARCH"

    apt_install minisign curl ca-certificates   # bootstrap-verify deps
    fetch_and_verify
    determine_src_dir

    install_runtime_deps
    enable_bluetooth
    check_networkmanager
    set_hostname
    install_polkit
    install_units
    install_labwc
    detect_hdmi_connector
    configure_display
    seed_config
    configure_ap
    # Last: pulls in unattended-upgrades, whose apt-daily timer can otherwise
    # contend with the install's own apt work.
    configure_unattended_upgrades
    enable_and_finish
}

main "$@"
