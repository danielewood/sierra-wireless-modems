# EM7455 GPS/GNSS Reference

## Quick Start

```bash
# 1. Unlock engineering commands
sudo ./swtool at 'AT!ENTERCND="A710"' --no-dry-run

# 2. Configure GPS (only needed once — persists across reboots)
sudo ./swtool at 'AT!CUSTOM="GPSENABLE",1' --no-dry-run
sudo ./swtool at 'AT!CUSTOM="GPSSEL",0' --no-dry-run      # 0=GNSS port, 1=AUX port
sudo ./swtool at 'AT!CUSTOM="GPSLPM",0' --no-dry-run       # keep GPS active in low-power mode
sudo ./swtool at 'AT!RESET' --no-dry-run                   # reboot to apply custom settings

# 3. After modem re-enumerates, unlock again and start tracking
sudo ./swtool at 'AT!ENTERCND="A710"' --no-dry-run
sudo ./swtool at 'AT!GPSTRACK=1,255,1000,1000,1' --no-dry-run

# 4. Check status (should show "Fix Session Status = ACTIVE")
sudo ./swtool at 'AT!GPSSTATUS?' --no-dry-run

# 5. Once fix acquired, query position
sudo ./swtool at 'AT!GPSLOC?' --no-dry-run
```

## Custom NVM Settings (`AT!CUSTOM?`)

| Setting | Values | Description |
|---------|--------|-------------|
| `GPSENABLE` | `0x00`=disabled, `0x01`=enabled | Enables the GPS/GNSS subsystem. `0x04` appears on some units but is non-standard; use `0x01`. |
| `GPSSEL` | `0x00`=GNSS port, `0x01`=AUX port | Selects which physical MHF4 antenna connector the GPS receiver listens on. **Must match your hardware wiring.** |
| `GPSLPM` | `0x00`=stay active, `0x01`=sleep in LPM | When set to 1 (default on many units), GPS shuts down whenever the modem enters low-power mode, preventing acquisition. **Set to 0.** |

> Changes to `AT!CUSTOM` settings require `AT!RESET` to take effect.

## Antenna Selection (`GPSSEL`)

The EM7455 M.2 card has three MHF4 connectors: **MAIN**, **AUX**, and **GNSS**.

- `GPSSEL=0` → dedicated **GNSS** antenna port (pin 22 on M.2 B-key)
- `GPSSEL=1` → shared **AUX** antenna port

Many laptops (ThinkPads, Dell Latitudes) only run two antenna cables (MAIN + AUX) to the WWAN slot, with no dedicated GNSS cable. In that case, use `GPSSEL=1`. If your setup has a dedicated GPS antenna on the GNSS port, use `GPSSEL=0`.

**If GPSSEL doesn't match which connector has the antenna, GPS will never acquire a fix.**

## Starting a GPS Session

GPS does **not** run automatically. You must explicitly start a session.

### One-Shot Fix (`AT!GPSFIX`)

```
AT!GPSFIX=<mode>,<timeout>,<accuracy>
```

| Param | Description | Example |
|-------|-------------|---------|
| mode | 1=standalone (no network assist) | `1` |
| timeout | Max seconds to wait (1–255) | `255` |
| accuracy | Desired accuracy in meters | `100` |

```
AT!GPSFIX=1,255,100
```

### Continuous Tracking (`AT!GPSTRACK`)

```
AT!GPSTRACK=<fixType>,<maxTime>,<maxDist>,<fixCount>,<fixRate>
```

| Param | Description | Example |
|-------|-------------|---------|
| fixType | 1=standalone | `1` |
| maxTime | Max seconds per fix attempt (1–255) | `255` |
| maxDist | Max acceptable error in meters | `1000` |
| fixCount | Number of fixes (1000=continuous) | `1000` |
| fixRate | Seconds between fixes | `1` |

```
AT!GPSTRACK=1,255,1000,1000,1
```

### Auto-Start on Boot (`AT!GPSAUTOSTART`)

```
AT!GPSAUTOSTART=1,1,255,100,1
```

Starts GPS tracking automatically whenever the modem boots. Useful for always-on GPS applications.

### Stop Session

```
AT!GPSEND=0
```

## Query Commands

| Command | Returns |
|---------|---------|
| `AT!GPSSTATUS?` | Fix status, session status, timestamps |
| `AT!GPSLOC?` | Lat/lon/alt/speed/heading (after fix) |
| `AT!GPSSATINFO?` | Visible satellite info (PRN, SNR, elevation, azimuth) |

## NMEA Output

NMEA sentences stream on the NMEA serial port (`/dev/ttyUSB1` in default MBIM composition).

```bash
# Activate NMEA streaming (from shell)
echo '$GPS_START' > /dev/ttyUSB1

# Read NMEA data
cat /dev/ttyUSB1
```

### NMEA Configuration

```
AT!GPSNMEACONFIG=1,1          # Enable NMEA output on USB NMEA interface
AT!GPSNMEASENTENCE=1F         # GPS-only NMEA sentences
AT!GPSNMEASENTENCE=C9FDF      # All constellations (GPS+GLONASS+Galileo+BeiDou)
```

## Other Useful Commands

| Command | Purpose |
|---------|---------|
| `AT!GPSPOSMODE=3F` | Enable positioning modes (bitmask: standalone + all assisted) |
| `AT+WANT=1` | Supply 3V to active GPS antenna (0=passive, 1=active) |
| `AT!GPSCLRASSIST` | Clear cached almanac/ephemeris (force cold start) |

## Troubleshooting

### GPS shows "NONE" for fix and session, date is 1980-01-06

The 1980-01-06 date is the **GPS epoch** (time zero). This means GPS has never acquired a fix. The GPS subsystem is idle — no session has been started.

**Fix:** Start a tracking session with `AT!GPSTRACK=1,255,1000,1000,1`.

### Session is ACTIVE but no fix after 60+ seconds

1. **Wrong antenna port** — verify `GPSSEL` matches your physical wiring
2. **No antenna connected** — the EM7455 has no internal GPS antenna; an external one is required
3. **No sky view** — GPS signals don't penetrate buildings or metal enclosures well; test near a window or outdoors
4. **GPSLPM=1** — GPS may be shutting down during brief low-power transitions; set `GPSLPM=0`

### Cold start vs warm start

| Start Type | Typical TTFF | When |
|------------|-------------|------|
| Cold | 30–60+ sec | First fix ever, or after `AT!GPSCLRASSIST` |
| Warm | 5–15 sec | Cached almanac/ephemeris from recent fix |
| Hot | 1–5 sec | Fix within last few minutes |

### GPS vs cellular independence

GPS operates independently of the cellular radio. The modem does **not** need to be registered on a network for standalone GPS. However, it must be powered on (`AT+CFUN=1`, not minimum functionality mode).

## Sources

- [Techship: Standalone GNSS Guide for EM/MC74xx](https://techship.com/support/faq/basic-standalone-gnss-gps-usage-guide-for-sierra-wireless-airprime-em-mc74-series-cellular-modules/)
- [Neilzone: EM7455 in ThinkPad with Debian 12 (GPS section)](https://neilzone.co.uk/2024/01/getting-the-sierra-wireless-em7455-lte-modem-working-in-a-thinkpad-with-debian-12-linux-with-gps/)
- [AirPrime EM74xx/MC74xx AT Command Reference (PDF)](https://source.sierrawireless.com/~/media/support_downloads/airprime/74xx/4117727_airprime_em74xx-mc74xx_at_command_reference_r3.ashx)
- Sierra Wireless Developer Forum (various threads on EM7455/MC7455 GPS)
