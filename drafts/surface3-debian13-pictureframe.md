# Picture frame su Microsoft Surface 3 (2 GB) — documento di lavoro

**Obiettivo:** trasformare il Surface 3 in un digital photo frame headless che esegue
`picture-frame` (Go + Svelte) in kiosk, senza desktop e con il touch funzionante.

**Hardware:** Surface 3 (non-Pro), Intel Atom x7-Z8700, **2 GB RAM**, eMMC 64/128 GB,
pannello **1920×1280** su `eDP-1`, touchscreen `surface3-spi`, WiFi Marvell 88W8897 (`mwifiex`).

**Stato:** installazione non ancora avviata. Media di installazione pronto (Debian 13.7 netinst).

---

## 1. Decisione finale

> **Debian 13 (trixie) minimal amd64 + kernel `linux-surface` + stack kiosk del progetto
> (`labwc` + `cog` + `seatd` + `wlopm`), nessun desktop, zram, suspend disabilitata.**

### Perché questa distro

| Distro | Verdetto | Motivo |
|---|---|---|
| **Debian 13 trixie minimal** | ✅ **scelta** | `cog`, `labwc`, `wlopm`, `seatd`, `wlr-randr` tutti in `main`. Stessa famiglia del target di test del progetto (Pi OS **Trixie** Lite). RAM minima |
| Xubuntu 24.04 minimal | ❌ | Ubuntu noble **non ha `cog`** (servirebbe compilarlo). Porta Xfce + LightDM: +600–800 MB |
| Ubuntu 24.04 | ❌ | Stessa assenza di `cog` |
| Fedora | ⚠️ | Kernel ottimo, ma `cog`/`labwc` da sistemare a mano; progetto mai testato |
| Debian testing/sid | ❌ | Il bug di `libwacom-surface` (#2076) colpisce proprio quelle suite |
| Raspberry Pi OS | ❌ | ARM: non gira su x86-64 |
| Alpine / Void | ❌ | Musl + init diverso; `deploy/` presume systemd + NetworkManager |

### Scheda tecnica

| Livello | Scelta | Perché |
|---|---|---|
| **Base** | Debian 13.7 netinst minimal, amd64, solo *standard system utilities* | Nessun desktop, nessun Xorg, nessun servizio superfluo |
| **Kernel** | `linux-image-surface` **6.19.8-surface-3** (repo `pkg.surfacelinux.com`, suite `release`) | Contiene il fix del touchscreen (`patches/6.19/0002-surface3.patch`) e i quirk WiFi (`0003-mwifiex.patch`). Il kernel Debian stock resta come voce GRUB di emergenza |
| **Init** | systemd, nessun display manager | `labwc` → `kiosk-backend` → `kiosk-browser`, avviati dai servizi del progetto |
| **Grafica** | Wayland puro: `labwc` + `cog` (WebKitGTK) | La wiki linux-surface: *"Wayland ha supporto nativo al touch, X11 emula un mouse"*. È il caso d'uso previsto dal progetto |
| **Output** | `eDP-1`, 1920×1280 | Pannello interno |
| **Spegnimento schermo** | `wlopm` (DPMS) via socket labwc | Fallback: luminosità 0 se il DPMS su eDP Intel delude |
| **Rete** | NetworkManager + `firmware-libertas` | Confermato da due fonti indipendenti come pacchetto giusto per il WiFi del Surface 3 |
| **Memoria** | zram (ram/2, zstd), `vm.swappiness=150`, `vm.page-cluster=0` | Obbligatorio con 2 GB: evita l'eMMC come swap |
| **Risparmio** | **suspend disabilitata**, `HandleLidSwitch=ignore` | Il crash mwifiex (#1965) ha come trigger principale il *resume* |
| **Accesso** | SSH + Tailscale | Manutenzione da remoto senza pagare RAM per un desktop |
| **Sicurezza** | Password admin dell'app + tailnet, nessuna porta esposta | Doc del progetto: *"never expose it to the internet"* |

### Budget RAM (2 GB)

| Voce | Costo |
|---|---|
| systemd + NM + bluez + avahi | 150–250 MB |
| labwc | 50–100 MB |
| cog + WebKitWebProcess | 250–450 MB |
| backend Go | 40–80 MB |
| tailscaled | 40–70 MB |
| **Totale** | **~530–950 MB** (margine ~1 GB) |
| *Xfce + LightDM (esclusi)* | *+600–800 MB* |

---

## 2. Cosa NON installare

| Pacchetto | Motivo |
|---|---|
| `iptsd` | È per **IPTS** (Surface Pro 4+). Il Surface 3 usa `surface3-spi` |
| `libwacom-surface` | Bug su Debian recenti (#2076), inutile con `labwc` |
| `linux-surface-secureboot-mok` | Serve solo con Secure Boot attivo (qui è disattivato) |
| **TLP** | Avviso ufficiale linux-surface: lag e freeze |
| Xfce / LightDM | Bruciano i 2 GB di RAM |
| App Electron (Obsidian, Joplin…) | 300–500 MB di base, non in Debian |

---

## 3. Lista maestra (fasi operative)

### FASE 0 — Media di installazione

- [ ] Flash completato con esito `VERIFICA OK -- chiavetta pronta`

```bash
# comando già eseguito (riferimento)
sudo bash /tmp/opencode/write-iso-to-usb.sh \
  debian-13.7.0-amd64-netinst.iso \
  a7ef94ac2fb9a7fec454552abd629b7cc9d5155c886165a45649f5ce6167e355 \
  /dev/sdX
```

**Riferimenti verificati**

| Voce | Valore |
|---|---|
| ISO | `debian-13.7.0-amd64-netinst.iso` — 792.723.456 byte |
| sha256 | `a7ef94ac2fb9a7fec454552abd629b7cc9d5155c886165a45649f5ce6167e355` |
| Chiavetta | 4.027.580.416 byte (3,75 GiB) |
| Firmware WiFi nell'installer | ✅ `firmware-libertas` sul media (contiene `mrvl/pcie8897_uapsta.bin`) |

> ⚠️ Il **pre-flight test in live** (Xubuntu) è saltato perché la chiavetta è stata
> sovrascritta. I due controlli equivalenti si fanno **dentro l'installer** (Fase 2a).

---

### FASE 1 — Surface: preparazione e boot

- [ ] Alimentatore collegato (Surface Connect: porta separata, la USB resta libera)
- [ ] Type Cover agganciata **prima** di accendere
- [ ] Chiavetta nell'unica porta USB, nient'altro collegato
- [ ] Surface spento → tieni premuto **Volume Giù** → premi **Power** → tieni Volume Giù **15–20 s**
- [ ] Compare GRUB Debian → scegli **`Install`**

**Piano B (boot):** **Power + Volume Su** → *UEFI Surface* → **Boot configuration** →
metti **USB Storage** in cima (o "Boot this device now").

---

### FASE 2 — Installazione Debian

#### 2a. Due controlli prima di toccare il disco

`Alt+F2` poi `Enter` (shell):

```bash
# a) touchscreen e Type Cover visti dal kernel?
cat /proc/bus/input/devices | grep -i name

# b) DMI rotto (OEMB)?
grep . /sys/devices/virtual/dmi/id/* 2>/dev/null | grep OEM | grep -v -e modalias -e uevent
```

- Deve comparire **`Surface3 SPI Touchscreen`** + i device della Type Cover.
- Se esce **`OEMB`**: il kernel linux-surface è obbligatorio (audio + tasti Fn). Se è vuoto, meglio.

`exit` per tornare al menu.

#### 2b. Piano B — installer bloccato sul rilevamento rete

Documentato sul campo (HOWTO Debian on Surface 3): *"the installation might hang at the
network hardware detection step"*.

```bash
# da un'altra console virtuale (Alt+F2)
killall ethdetect      # prosegui senza configurare la rete
# la rete la configuriamo dopo con NetworkManager (Fase 5)
```

#### 2c. Scelte dell'installer

| Schermata | Scelta |
|---|---|
| Lingua / paese / tastiera | Italiano, Italia, Italiano |
| WiFi | la tua rete (grazie a `firmware-libertas`) |
| "missing firmware / load from media?" | **Sì** |
| Hostname | `pictureframe` |
| Dominio | vuoto |
| Password di root | **vuota** (si usa `sudo`) |
| Nome utente / password | i tuoi |
| Fuso orario | Europe/Rome |
| Partizionamento | **Guidato – usa l'intero disco** |
| **Quale disco** | ⚠️ l'**eMMC interno** (≈58 o ≈119 GB) — **NON** `sdX` "USB DISK SMI" (3,8 GB) |
| Mirror | `deb.debian.org`, proxy vuoto |
| popcon | no |
| Software selection | **solo "standard system utilities"** — **nessun desktop** |
| GRUB | sì, sul disco interno |

---

### FASE 3 — Primo avvio ⛳ *checkpoint*

- [ ] Raccogliere e salvare l'output:

```bash
uname -r
nmcli device status 2>/dev/null || ip -br addr
cat /proc/bus/input/devices | grep -i name
ls /sys/class/backlight/
cat /etc/network/interfaces
lsblk
```

Attesi: `uname -r` = kernel Debian 6.12.x; `wlan0` presente; `intel_backlight` in
`/sys/class/backlight/`; eMMC visibile come `mmcblk0`.

---

### FASE 4 — Kernel linux-surface + fondamenta

#### 4a. Kernel, microcodice, firmware

```bash
# chiave e repository linux-surface (comandi ufficiali)
wget -qO - https://raw.githubusercontent.com/linux-surface/linux-surface/master/pkg/keys/surface.asc \
    | gpg --dearmor | sudo dd of=/etc/apt/trusted.gpg.d/linux-surface.gpg

echo "deb [arch=amd64] https://pkg.surfacelinux.com/debian release main" \
    | sudo tee /etc/apt/sources.list.d/linux-surface.list

sudo apt update

# NOTA: niente iptsd, niente libwacom-surface, niente secureboot-mok
sudo apt install -y linux-image-surface linux-headers-surface \
                    intel-microcode firmware-libertas firmware-sof-signed

sudo update-grub && sudo reboot
```

#### 4b. Verifiche post-riavvio

```bash
uname -r                                   # deve contenere "surface"
ls /sys/class/backlight/                   # deve mostrare intel_backlight
cat /sys/module/surface3_spi/parameters/use_dma   # atteso: N (DMA disattivato = niente crash)
sudo dmesg | grep -iE 'surface3|SPI transfer timed out'
free -h
```

- [ ] `uname -r` contiene `surface`
- [ ] `intel_backlight` presente
- [ ] `use_dma` = `N`
- [ ] nessun `SPI transfer timed out` nei log

#### 4c. zram (2 GB: obbligatorio)

```bash
sudo apt install -y systemd-zram-generator

sudo tee /etc/systemd/zram-generator.conf >/dev/null <<'EOF'
[zram0]
zram-size = ram / 2
compression-algorithm = zstd
EOF

sudo tee /etc/sysctl.d/99-picture-frame.conf >/dev/null <<'EOF'
vm.swappiness = 150
vm.page-cluster = 0
EOF

sudo systemctl daemon-reload && sudo reboot
```

Verifica: `zramctl` deve mostrare `zram0` attivo.

#### 4d. Suspend disabilitata (contromisura al crash mwifiex)

Il trigger principale della caduta WiFi è il *resume from suspend*
([linux-surface#1965](https://github.com/linux-surface/linux-surface/issues/1965)).
Il frame deve restare acceso: lo schermo lo spegne il DPMS, non il sistema.

```bash
sudo systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target

sudo sed -i 's/^#\?HandleLidSwitch=.*/HandleLidSwitch=ignore/' /etc/systemd/logind.conf
sudo sed -i 's/^#\?HandleSuspendKey=.*/HandleSuspendKey=ignore/' /etc/systemd/logind.conf
sudo systemctl restart systemd-logind
```

- [ ] `systemctl status sleep.target` → *masked*

---

### FASE 5 — Rete, trasferimento, installazione del frame ⛳ *checkpoint*

#### 5a. NetworkManager + SSH

```bash
sudo apt install -y network-manager openssh-server

# NM deve prendere in carico wlan0: togli la config di ifupdown
sudo tee /etc/network/interfaces >/dev/null <<'EOF'
auto lo
iface lo inet loopback
EOF

sudo systemctl enable --now NetworkManager
sudo nmcli device wifi list
sudo nmcli device wifi connect "IL_TUO_SSID" password "LA_TUA_PASSWORD"
nmcli device status          # wlan0 = connected
ip -br addr                  # annota l'IP
```

> ⚠️ `deploy/install.sh` **richiede NetworkManager attivo** e con la rete funzionante.

#### 5b. Tailscale (accesso remoto e manutenzione)

```bash
curl -fsSL https://pkgs.tailscale.com/stable/debian/trixie.noarmor.gpg \
  | sudo tee /usr/share/keyrings/tailscale-archive-keyring.gpg >/dev/null
curl -fsSL https://pkgs.tailscale.com/stable/debian/trixie.tailscale-keyring.list \
  | sudo tee /etc/apt/sources.list.d/tailscale.list

sudo apt update && sudo apt install -y tailscale

sudo tailscale up --ssh        # apri l'URL dal telefono, oppure usa una auth key
tailscale status
tailscale ip -4
```

Consigliato: **niente** exit node / subnet router. Utile da subito: da qui puoi guidare
il Surface via SSH invece di digitare sulla Type Cover.

#### 5c. Trasferimento del progetto (dal PC, non dal Surface)

La release ufficiale `v1.3.0` **non ha l'asset amd64** (solo armv6/armv7/arm64), quindi si
usa il binario già compilato in `dist/picture-frame` (statico x86-64, dal commit
*"feat: add Linux x86-64 and Surface 3 support"*).

```bash
# sul PC, dalla cartella del progetto
scp -r dist/picture-frame deploy config.example.toml rmaistri@<ip-surface>:~/pf-staging/
```

#### 5d. Installazione (sul Surface)

```bash
cd ~/pf-staging

# anteprima (facoltativa ma consigliata)
sudo bash deploy/install.sh --dry-run --version v1.3.0 --display-output eDP-1 --no-ap

# payload precaricato: così l'installer SALTA il download (che per amd64 non esiste)
DEST="$HOME/picture-frame"
mkdir -p "$DEST"
install -m 0755 dist/picture-frame  "$DEST/picture-frame"
install -m 0644 config.example.toml "$DEST/config.example.toml"
printf 'v1.3.0\n' > "$DEST/.release"

sudo bash deploy/install.sh --version v1.3.0 --display-output eDP-1 --no-ap
```

L'installer provvede a: pacchetti (`seatd cog labwc wlopm wlr-randr bluez avahi-daemon
dnsmasq-base`), servizi systemd, `config.toml` con `output = "eDP-1"`, dir `labwc/`.
Chiede la **password admin dell'app**. `--no-ap` salta l'hotspot (il WiFi c'è).

> **Nota tecnica (perché funziona):** `install.sh` usa `payload_is_current()` — se in
> `INSTALL_DIR` esistono il binario eseguibile **e** il marker `.release` uguale al tag
> richiesto, salta download e verifica minisign. Il percorso è documentato nel codice.
> `--version v1.3.0` richiede solo che il tag esista su GitHub (chiamata API).

> **Alternativa pulita:** pubblicare una release con l'asset **amd64** (già previsto in
> `.goreleaser.yaml`). Da quel momento sul Surface basta:
> `curl -fsSL https://github.com/MateEke/picture-frame/releases/latest/download/install.sh | sudo bash -s -- --display-output eDP-1 --no-ap`

- [ ] Servizi attivi: `labwc`, `kiosk-backend`, `kiosk-browser`
- [ ] Lo schermo mostra il frame (orologio; le foto dopo il primo upload)

---

### FASE 6 — Test sul campo

#### 6a. Servizi e pannello

```bash
systemctl status labwc kiosk-backend kiosk-browser --no-pager

# DPMS (il test più incerto su Intel/eDP: il progetto l'ha validato solo su Pi VC4)
XDG_RUNTIME_DIR=/run/picture-frame WAYLAND_DISPLAY=wayland-0 wlopm --off eDP-1
XDG_RUNTIME_DIR=/run/picture-frame WAYLAND_DISPLAY=wayland-0 wlopm --on eDP-1
```

- [ ] `wlopm --off` spegne davvero il pannello
- [ ] `wlopm --on` lo riaccende
- **Se fallisce:** `blank_after = "0s"` in `config.toml` + spegnimento via luminosità 0.

#### 6b. Touch (test prolungato: 5 minuti di tap)

```bash
# watch dei log mentre tocchi
sudo dmesg -w | grep -iE 'surface3|SPI transfer timed out'
```

- [ ] Tocco metà destra → foto successiva
- [ ] Tocco metà sinistra → foto precedente
- [ ] Tocco a schermo spento → risveglia sulla stessa foto
- [ ] Dopo **5 minuti** di tap continui: **nessun** `SPI transfer timed out`
- **Recupero a caldo se il driver si impunta:**
  ```bash
  sudo modprobe -r spi_pxa2xx_platform && sudo modprobe spi_pxa2xx_platform
  ```

#### 6c. Audio, batteria, SD

```bash
speaker-test -c2 -twav -l1        # audio (firmware-sof-signed già installato)
ls /sys/class/power_supply/        # batteria (surface3_power)
lsblk                              # eMMC + eventuale microSD
```

- **Se l'audio fa un tono singolo:** aggiungi `snd_intel_dspcfg.dsp_driver=2` al cmdline.
- La **microSD** è un buon posto per le foto (il reader funziona).

#### 6d. Watchdog WiFi (contromisura alle cadute mwifiex)

```bash
sudo tee /usr/local/sbin/pf-wifi-watchdog.sh >/dev/null <<'EOF'
#!/bin/bash
# Se wlan0 sparisce (driver mwifiex morto), ricarica il modulo.
set -u
DEV=wlan0
if ! ip link show "$DEV" >/dev/null 2>&1; then
    logger -t pf-wifi-watchdog "$DEV sparita: ricarico mwifiex"
    modprobe -r mwifiex_pcie mwifiex cfg80211 2>/dev/null || true
    sleep 2
    modprobe mwifiex_pcie || true
    systemctl restart NetworkManager || true
fi
EOF
sudo chmod 0755 /usr/local/sbin/pf-wifi-watchdog.sh

sudo tee /etc/systemd/system/pf-wifi-watchdog.service >/dev/null <<'EOF'
[Unit]
Description=Picture frame WiFi watchdog (mwifiex recovery)

[Service]
Type=oneshot
ExecStart=/usr/local/sbin/pf-wifi-watchdog.sh
EOF

sudo tee /etc/systemd/system/pf-wifi-watchdog.timer >/dev/null <<'EOF'
[Unit]
Description=Run the picture frame WiFi watchdog every 2 minutes

[Timer]
OnBootSec=3min
OnUnitActiveSec=2min

[Install]
WantedBy=timers.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now pf-wifi-watchdog.timer
```

#### 6e. Riavvio e uso reale

- [ ] Riavvio → il frame riparte da solo, senza login
- [ ] Da telefonino: `http://<ip>/` (o `http://pictureframe/` via Tailscale) → carico le foto
- [ ] Slideshow, orologio, sensori (via MQTT/HA) funzionanti

---

## 4. Rischi e contromisure

| # | Rischio | Evidenza | Contromisura |
|---|---|---|---|
| 1 | **Touchscreen SPI che crasha** | [#1184](https://github.com/linux-surface/linux-surface/issues/1184) | Il kernel 6.19 include il workaround (DMA off di default). Recupero a caldo con `modprobe -r/ins spi_pxa2xx_platform` |
| 2 | **WiFi mwifiex che muore** | [#1965](https://github.com/linux-surface/linux-surface/issues/1965) | Suspend disabilitata + watchdog WiFi + riconnessione automatica Tailscale |
| 3 | **DPMS su eDP Intel non validato** | Il progetto l'ha testato solo su Pi VC4 | Test in Fase 6a; fallback `blank_after = "0s"` + luminosità 0 |
| 4 | **Touch "ogni 5 tocchi"** (report su Fedora 41 / 6.12) | [#1184 commenti](https://github.com/linux-surface/linux-surface/issues/1184) | Sul frame il gesto è solo tap (non drag): impatto minimo; il pannello admin resta sul telefono |
| 5 | **Installer bloccato su rilevamento rete** | HOWTO 2foo.net (2016) | Fase 2b: `killall ethdetect`, rete configurata dopo con NM |
| 6 | **Asset amd64 assente nelle release** | `v1.3.0` ha solo armv6/armv7/arm64 | Fase 5d: payload locale. L'auto-updater, se non trova l'asset per la piattaforma, logga a livello debug e **non tocca** il binario |

---

## 5. Evidenze raccolte (fonti)

### Patch del kernel (prova diretta)

Nel repo `linux-surface/linux-surface`, set **`patches/6.19/`** — quello del kernel
`6.19.8-surface-3` che installiamo:

- **`0002-surface3.patch`** contiene due cose:
  1. `(surface3-oemb) add DMI matches for Surface 3 with broken DMI` →
     `surface3-wmi.c`, `sound/soc/codecs/rt5645.c`, `soc-acpi-intel-cht-match.c`
  2. `surface3-spi: workaround: disable DMA mode to avoid crash by`
     → parametro `use_dma` **default `false`** (PIO), con nota:
     *"This patch was originally dropped... However, according to the comments in [4] it is still required."* (2025-03-08)
- **`0003-mwifiex.patch`** → quirk Marvell 88W8897 (reset via `_DSM`, ASPM L1/substati off, niente host-sleep)
- Presenti anche: `0005-ipts`, `0006-ithc`, `0010-surface-typecover`, `0013-cameras`, ecc.

### Issue rilevanti

| Issue | Titolo | Stato |
|---|---|---|
| [#1184](https://github.com/linux-surface/linux-surface/issues/1184) | surface 3 touchscreen driver crash randomly | aperta |
| [#1965](https://github.com/linux-surface/linux-surface/issues/1965) | Surface 3, mwifiex/_pcie dies...sometimes | aperta (gen 2026) |
| [#681](https://github.com/linux-surface/linux-surface/issues/681) | No microphone on Surface 3 (Non-Pro) | aperta |
| [#209](https://github.com/linux-surface/linux-surface/issues/209) | Cameras not working | aperta |
| [#2076](https://github.com/linux-surface/linux-surface/issues/2076) | `libwacom-surface` rompe il desktop (Debian testing/sid, Ubuntu 26.04) | aperta |
| [#2221](https://github.com/linux-surface/linux-surface/issues/2221) | Kernel image name on Debian incompatibile con debian live-build | aperta (ago 2026) |

### Wiki linux-surface

- **Surface 3**: touchscreen (`surface3-spi`), OEMB, audio rt5645/SOF, batteria `surface3_power`,
  backlight (`i915` deve essere modulo), S0ix, camere, LTE, TLP da evitare.
- **Marvell 88W8897 quirks**: per il Surface 3 card reset via `_DSM` + ASPM L1 e substati disabilitati.
- **Touch controls**: *"Wayland has native support for touch inputs, while Xorg essentially emulates a mouse pointer"*.

### Esperienze di campo

- **HOWTO "Debian on Surface 3 (non-pro)"** (2016, recuperato da
  [Wayback](https://web.archive.org/web/20160630023709/http://www.2foo.net/howto-surface-3-on-debian-linux/)):
  *"Wifi works after installing the `firmware-libertas` package and rebooting"*;
  installer che può bloccarsi su `ethdetect`; disabilitare suspend per evitare freeze.
- **Blog di campo** (Surface 3 Atom Z8700, Debian): basta il kernel linux-surface + `firmware-libertas`.
- **Installation wiki** (linux-surface): comandi Debian/Ubuntu usati in Fase 4a; per
  Debian testing/sid **non** installare `libwacom-surface`; `iptsd` solo da SP4 in su.
- ⚠️ **Trappola nelle ricerche:** gran parte dei risultati "Surface 3" è in realtà
  **Surface Pro 3** (i5, touch IPTS → richiede `iptsd`). Non applicabile qui.

### Verifiche fatte in locale

| Voce | Esito |
|---|---|
| Pacchetti in Debian 13 `main` | ✅ `cog`, `labwc`, `wlopm`, `seatd`, `network-manager`, `openssh-server`, `systemd-zram-generator`, `alsa-utils` |
| Firmware in `non-free-firmware` | ✅ `firmware-libertas`, `firmware-sof-signed`, `intel-microcode` |
| Linux-surface Debian repo | ✅ suite `release`, `linux-image-surface` = 6.19.8-surface-3 |
| Tailscale per trixie | ✅ repo presente, 1.104.1 |
| Release `picture-frame` | `v1.3.0` = armv6/armv7/arm64 (**niente amd64**) |
| Binario locale | `dist/picture-frame`, ELF 64-bit statically linked, x86-64 |
| `install.sh` su x86 | ✅ salta `config.txt` ("standard UEFI/x86 host"), `rfkill` protetto da `command -v` |
| Auto-updater con asset mancante | ✅ log debug, nessuna modifica al binario |

---

## 6. Appendice

### A. Perché il payload locale funziona (dettaglio)

`deploy/install.sh`:

```
payload_is_current() {
    [ -x "$INSTALL_DIR/picture-frame" ] || return 1
    [ "$(cat "$INSTALL_DIR/$PAYLOAD_MARKER")" = "$RELEASE_TAG" ]
}
```

- `INSTALL_DIR` default: `$HOME/picture-frame` dell'utente che invoca `sudo`
- `PAYLOAD_MARKER` = `.release`
- Se il controllo passa, `fetch_and_verify()` ritorna prima del download → nessuna verifica minisign
  (accettabile: il binario è compilato in locale)
- `SRC_DIR` = `deploy/` del repo, perché lo script viene eseguito da lì (contiene `systemd/`)

### B. Variante con desktop (opzionale, non consigliata su 2 GB)

Se servisse un desktop per **note a penna** (`xournalpp`) o PDF (`zathura`):

- Aggiungere un session manager (`greetd`) con due voci: **Kiosk** e **Xfce**
- Costo in **disco**, non in RAM: le sessioni non girano insieme
- Alternativa più leggera: usare l'app **dentro** la sessione labwc esistente:
  ```bash
  sudo systemctl stop kiosk-browser.service
  XDG_RUNTIME_DIR=/run/picture-frame WAYLAND_DISPLAY=wayland-0 imv -f ~/nota.png
  sudo systemctl start kiosk-browser.service
  ```
- App leggere verificate in trixie: `mousepad`, `zathura`(+pdf), `imv`, `swayimg`, `foot`,
  `cherrytree`, `zim`, `xournalpp`, tastiere a schermo `wvkbd`/`squeekboard`/`matchbox-keyboard`

### C. Paracadute

| Situazione | Comando |
|---|---|
| Boot col kernel surface problematico | GRUB → voce **Debian stock** |
| Touch impuntato | `sudo modprobe -r spi_pxa2xx_platform && sudo modprobe spi_pxa2xx_platform` |
| WiFi morto | `sudo systemctl restart pf-wifi-watchdog.service` |
| Rimuovere il frame | `sudo bash deploy/install.sh --uninstall` |
| DPMS non funziona | `blank_after = "0s"` + luminosità 0 |

### D. Comandi utili

```bash
# stato frame
systemctl status labwc kiosk-backend kiosk-browser --no-pager
journalctl -u kiosk-browser -n 50 --no-pager

# touch
cat /sys/module/surface3_spi/parameters/use_dma
sudo dmesg | grep -iE 'surface3|mwifiex'

# rete
nmcli device status ; tailscale status

# memoria
free -h ; zramctl
```
