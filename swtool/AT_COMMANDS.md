# AT Command Reference — Sierra Wireless EM7455/MC7455/EM7565

Master reference for all AT commands used with Sierra Wireless AirPrime EM74xx/MC74xx modems.

Official Sierra Wireless reference: [AirPrime EM74xx/MC74xx AT Command Reference (PDF)](https://source.sierrawireless.com/~/media/support_downloads/airprime/74xx/4117727_airprime_em74xx-mc74xx_at_command_reference_r3.ashx)

---

## Prerequisites

All `AT!` engineering commands require unlocking first:

```
AT!ENTERCND="A710"
```

This must be sent before any `AT!` set/write commands. Query commands (`AT!xxx?`) work without it but set commands will return `ERROR`.

---

## 1. Identity & Information

### `AT`

Test command. Verifies the modem is responsive and the serial connection works.

- **Response:** `OK`

### `ATE1` / `ATE0`

Enable (`ATE1`) or disable (`ATE0`) local echo. When enabled, the modem echoes back every character sent to it.

- **Response:** `OK`

### `ATI`

Display modem identification. Returns manufacturer, model, firmware revision, and device serial numbers.

```
Manufacturer: Sierra Wireless, Incorporated
Model: EM7455B
Revision: SWI9X30C_02.24.05.06 r7040 CARMD-EV-FRMWR2 2017/05/19 06:23:09
MEID: 35448008243466
IMEI: 354480082434669
IMEI SV: 12
FSN: LF103291240310
+GCAP: +CGSM
```

| Field | Description |
|-------|-------------|
| Manufacturer | Always "Sierra Wireless, Incorporated" |
| Model | Hardware model (EM7455, EM7455B, MC7455, EM7565) |
| Revision | Firmware version + build info + date |
| MEID | Mobile Equipment Identifier (CDMA) |
| IMEI | International Mobile Equipment Identity (GSM/LTE) |
| IMEI SV | IMEI Software Version |
| FSN | Factory Serial Number (unique per unit) |
| +GCAP | General Capabilities |

### `AT!HWID?`

Query hardware revision identifier.

```
Revision: 0.5
```

### `AT!USBINFO?`

Query USB descriptor information (VID, PIDs, manufacturer string, product string).

```
VID: 0x413C
APP PID: 0x81B6
BOOT PID: 0x81B5
Manufacturer: Sierra Wireless, Incorporated
Product: DW5811e Snapdragon X7 LTE
```

---

## 2. USB Configuration

### `AT!USBCOMP?` / `AT!USBCOMP=?`

Query current USB composition or list valid options.

```
Config Index: 1
Config Type:  1 (Generic)
Interface bitmask: 0020100D (diag,nmea,modem,mbim,ubist)
```

### `AT!USBCOMP=<index>,<type>,<bitmask>`

Set USB interface composition. Determines which USB interfaces the modem exposes to the host.

| Parameter | Values | Description |
|-----------|--------|-------------|
| Config Index | `1` | Always 1 |
| Config Type | `1` = Generic, `2` = USBIF-MBIM, `3` = RNDIS | Use `1` for custom VID/PID, `3` for EM7565 |
| Interface bitmask | Hex OR of interfaces | See table below |

**Interface bitmask values:**

| Interface | Bit | Hex Value |
|-----------|-----|-----------|
| DIAG | 0 | `0x00000001` |
| ADB | 1 | `0x00000002` |
| NMEA | 2 | `0x00000004` |
| MODEM | 3 | `0x00000008` |
| RMNET0 | 8 | `0x00000100` |
| RMNET1 | 10 | `0x00000400` |
| RMNET2 | 11 | `0x00000800` |
| MBIM | 12 | `0x00001000` |
| RNDIS | 14 | `0x00004000` |
| AUDIO | 16 | `0x00010000` |
| ECM | 19 | `0x00080000` |
| UBIST | 21 | `0x00200000` |

**Common compositions:**

| Composition | Bitmask | Interfaces | Use case |
|-------------|---------|------------|----------|
| MBIM mode | `0000100D` | diag, nmea, modem, mbim | Default for most Linux setups |
| QMI mode | `0000010D` | diag, nmea, modem, rmnet0 | Alternative network interface |
| MBIM + UBIST | `0020100D` | diag, nmea, modem, mbim, ubist | Dell default |

**Examples:**

```
AT!USBCOMP=1,1,0000100D    # EM7455: MBIM mode (recommended)
AT!USBCOMP=1,1,0000010D    # EM7455: QMI mode
AT!USBCOMP=1,3,0000100D    # EM7565: MBIM mode (note config type 3)
```

> RMNET0 and MBIM cannot be used simultaneously.

### `AT!USBVID?` / `AT!USBVID=<hex>`

Query or set the USB Vendor ID.

| VID | Manufacturer |
|-----|-------------|
| `1199` | Sierra Wireless (generic) |
| `413C` | Dell |

### `AT!USBPID?` / `AT!USBPID=<app>,<boot>`

Query or set the USB Product IDs. Two PIDs are stored: one for normal operation (APP) and one for bootloader/QDL mode (BOOT).

| Identity | APP PID | BOOT PID |
|----------|---------|----------|
| Sierra Wireless EM7455 | `9071` | `9070` |
| Lenovo EM7455 | `9079` | `9078` |
| Dell DW5811e | `81B6` | `81B5` |

```
AT!USBPID=9071,9070    # Generic Sierra Wireless
AT!USBPID=9079,9078    # Lenovo
AT!USBPID=81B6,81B5    # Dell
```

### `AT!USBPRODUCT?` / `AT!USBPRODUCT="<string>"`

Query or set the USB product descriptor string (what the OS sees in device listings).

```
AT!USBPRODUCT="EM7455"
AT!USBPRODUCT="Sierra Wireless EM7455 Qualcomm Snapdragon X7 LTE-A"
AT!USBPRODUCT="Dell Wireless 5811e Gobi(TM)4G LTE Mobile Broadband Card"
```

### `AT!USBSPEED?` / `AT!USBSPEED=<0|1>`

Query or set the USB interface speed.

```
SUPPORTED:Super-Speed
CURRENT  :High-Speed
```

| Value | Speed | Notes |
|-------|-------|-------|
| `0` | USB 2.0 (High Speed) | Default. No performance loss — EM7455 LTE throughput fits within USB 2.0 bandwidth |
| `1` | USB 3.0 (SuperSpeed) | May cause issues with some M.2 slots |

---

## 3. Modem Identity Presets

Complete identity change sequences for converting between OEM brands:

**Generic Sierra Wireless:**
```
AT!USBVID=1199
AT!USBPID=9071,9070
AT!USBPRODUCT="EM7455"
AT!PRIID="9904609","002.026","Generic-Laptop"
```

**Lenovo:**
```
AT!USBVID=1199
AT!USBPID=9079,9078
AT!USBPRODUCT="Sierra Wireless EM7455 Qualcomm Snapdragon X7 LTE-A"
AT!PRIID="9904609","002.026","Lenovo-Storm"
```

**Dell DW5811e:**
```
AT!USBVID=413C
AT!USBPID=81B6,81B5
AT!USBPRODUCT="Dell Wireless 5811e Gobi(TM)4G LTE Mobile Broadband Card"
AT!PRIID="9904609","002.026","DELL"
```

---

## 4. Firmware & Image Management

### `AT!IMPREF?` / `AT!IMPREF="<carrier>"`

Query or set the preferred firmware/carrier image. The modem selects which carrier configuration to boot based on this preference.

```
!IMPREF:
 preferred fw version:    02.24.05.06
 preferred carrier name:  GENERIC
 preferred config name:   GENERIC_002.026_000
 current fw version:      02.24.05.06
 current carrier name:    GENERIC
 current config name:     GENERIC_002.026_000
```

**Common carrier names:** `GENERIC`, `ATT`, `VZW`, `SPRINT`, `TMO`, `AUTO-SIM`

```
AT!IMPREF="GENERIC"     # Use generic carrier profile
AT!IMPREF="AUTO-SIM"    # Auto-detect based on SIM card
```

> If preferred and current carrier names don't match, the modem may enter Low Power Mode. Use `AT!PCINFO?` to diagnose.

### `AT!GOBIIMPREF?` / `AT!GOBIIMPREF="<carrier>"`

Same as `AT!IMPREF` but for the GOBI firmware management layer. Send both for compatibility across firmware versions.

```
AT!GOBIIMPREF="GENERIC"
```

### `AT!IMAGE?`

List all firmware (FW) and carrier configuration (PRI) image slots.

```
TYPE SLOT STATUS LRU FAILURES UNIQUE_ID   BUILD_ID
FW   1    GOOD   1   0 0      ?_?         02.24.05.06_?
FW   2    EMPTY  0   0 0
FW   3    EMPTY  0   0 0
FW   4    EMPTY  0   0 0
Max FW images: 4
Active FW image is at slot 1

TYPE SLOT STATUS LRU FAILURES UNIQUE_ID   BUILD_ID
PRI  FF   GOOD   0   0 0      002.026_000 02.24.05.06_GENERIC
Max PRI images: 50
```

| Field | Description |
|-------|-------------|
| TYPE | `FW` = firmware, `PRI` = carrier configuration |
| SLOT | Slot number (1-4 for FW, FF for PRI) |
| STATUS | `GOOD` = valid, `EMPTY` = unused, `BAD` = corrupted |
| LRU | Least Recently Used counter (higher = more recently booted) |
| FAILURES | Number of failed boot attempts from this image |
| UNIQUE_ID | Image identifier |
| BUILD_ID | Firmware version + carrier tag |

### `AT!IMAGE=<op>[,<type>[,<slot>]]`

Manage firmware image slots.

| Op | Description |
|----|-------------|
| `0` | Delete images |
| `1` | List images |
| `2` | Get max number of images |

```
AT!IMAGE=0              # Delete ALL firmware and PRI images
AT!IMAGE=0,0,3          # Delete firmware in slot 3 only (type 0=FW)
AT!IMAGE=0,1,FF         # Delete PRI in slot FF only (type 1=CONFIG)
```

> After clearing images with `AT!IMAGE=0`, you must reset and re-flash firmware.

### `AT!PRIID?` / `AT!PRIID="<pn>","<rev>","<customer>"`

Query or set the PRI (Product Release Instruction) identification. This ties the modem to a specific carrier profile.

```
PRI Part Number: 9907375
Revision: 001.001
Customer: PebbleCreekMLK

Carrier PRI: 9999999_9904609_SWI9X30C_02.24.05.06_00_GENERIC_002.026_000
```

The Carrier PRI string is automatically derived from the NVU firmware file. The part number and revision must match the firmware. The customer name determines the OEM profile.

---

## 5. Network & Band Configuration

### `AT!SELRAT?` / `AT!SELRAT=<index>`

Query or set the Radio Access Technology selection.

```
!SELRAT: 06, LTE Only
```

| Index | Mode |
|-------|------|
| `00` | Automatic (all technologies) |
| `01` | UMTS 3G Only |
| `06` | LTE Only |
| `11` | UMTS and LTE Only |

### `AT!BAND?` / `AT!BAND=?`

Query current band or list all available band configurations.

```
Index, Name,                        GW Band Mask     L Band Mask      TDS Band Mask
00, All bands,                      0002000007C00000 00000100130818DF 0000000000000000
01, Europe 3G,                      0002000000400000 0000000000000000 0000000000000000
06, Europe,                         0002000000400000 00000000000800C5 0000000000000000
07, North America,                  0000000004800000 000000000300185A 0000000000000000
08, WCDMA ALL,                      0002000007C00000 0000000000000000 0000000000000000
09, LTE ALL,                        0000000000000000 00000100130818DF 0000000000000000
```

### `AT!BAND=<index>` / `AT!BAND=<slot>,"<name>",<gw_mask>,<lte_mask>`

Set band selection to a preset index or define a custom band lock.

```
AT!BAND=00                          # All bands (clear band lock)
AT!BAND=09                          # LTE-only bands
AT!BAND=10                          # Activate custom slot 10
```

**Custom band lock examples:**

```
AT!BAND=10,"B2 (1900)",0,0000000000000002         # Band 2 only
AT!BAND=10,"B4 (1700/2100)",0,0000000000000008    # Band 4 only
AT!BAND=10,"B5 (850)",0,0000000000000010          # Band 5 only
AT!BAND=10,"B12 (700)",0,0000000000000800         # Band 12 only
AT!BAND=10,"B2B4",0,000000000000000A              # Bands 2+4
AT!BAND=10,"B2B4B5B12",0,000000000000081A         # Bands 2+4+5+12
```

**LTE band bitmask reference:**

| Band | Hex Bit |
|------|---------|
| B1 (2100) | `0x0000000000000001` |
| B2 (1900) | `0x0000000000000002` |
| B3 (1800) | `0x0000000000000004` |
| B4 (1700/2100) | `0x0000000000000008` |
| B5 (850) | `0x0000000000000010` |
| B7 (2600) | `0x0000000000000040` |
| B8 (900) | `0x0000000000000080` |
| B12 (700) | `0x0000000000000800` |
| B13 (700) | `0x0000000000001000` |
| B20 (800) | `0x0000000000080000` |
| B25 (1900) | `0x0000000001000000` |
| B26 (850) | `0x0000000002000000` |
| B29 (700) | `0x0000000010000000` |
| B41 (2500) | `0x0000010000000000` |

To calculate a multi-band bitmask, OR the individual band values together.

---

## 6. Power Control

### `AT!PCINFO?`

Query power control state and Low Power Mode (LPM) voters. Critical for diagnosing modems stuck in low power mode.

```
State: Online
LPM voters - Temp:0, Volt:0, User:0, W_DISABLE:0, IMSWITCH:0, BIOS:0, LWM2M:0, OMADM:0, FOTA:0
LPM persistence - None
```

When stuck in Low Power Mode:

```
State: Low Power Mode
LPM voters - Temp:0, Volt:0, User:0, W_DISABLE:0, IMSWITCH:1, BIOS:0, LWM2M:0, OMADM:0, FOTA:0
LPM persistence - None
```

| Voter | Description |
|-------|-------------|
| Temp | Temperature out of range |
| Volt | Voltage out of range |
| User | User-requested power down |
| W_DISABLE | W_DISABLE pin asserted by host (see `AT!PCOFFEN`) |
| IMSWITCH | Image switch pending (firmware/carrier mismatch) |
| BIOS | BIOS-level power control |
| LWM2M | LWM2M management |
| OMADM | OMA-DM management |
| FOTA | Firmware OTA update pending |

### `AT!PCOFFEN?` / `AT!PCOFFEN=<0|1|2>`

Query or set the W_DISABLE pin behavior. Many laptops assert W_DISABLE on their internal M.2 WWAN slots.

| Value | Behavior |
|-------|----------|
| `0` | Normal — modem obeys W_DISABLE pin (default) |
| `1` | Power off when W_DISABLE asserted |
| `2` | **Ignore W_DISABLE** — modem stays powered on regardless. Required for ThinkPad X1G6, T470, and other laptops that aggressively disable non-whitelisted WWAN cards |

```
AT!PCOFFEN=2    # Ignore W_DISABLE pin (recommended for most laptop installs)
```

### `AT!RESET`

Save all pending settings to NVM and reboot the modem. Required after any configuration change. The modem disconnects from USB and re-enumerates.

> After `AT!RESET`, the serial port will close. Re-detect the modem before sending more commands.

---

## 7. Custom Settings

### `AT!CUSTOM?` / `AT!CUSTOM="<key>",<value>`

Query all custom NVM settings or set a specific one.

```
!CUSTOM:
             GPSENABLE          0x01
             GPIOSARENABLE      0x01
             GPSSEL             0x01
             IPV6ENABLE         0x01
             SIMLPM             0x01
             USBSERIALENABLE    0x01
             FASTENUMEN         0x02
             SINGLEAPNSWITCH    0x01
```

### `AT!CUSTOM="FASTENUMEN",<0-3>`

Fast enumeration mode. Controls whether the modem skips full initialization during USB enumeration.

| Value | Behavior |
|-------|----------|
| `0` | Disabled — full initialization every boot |
| `1` | Fast enum on cold boot, full init on warm boot |
| `2` | **Fast enum on warm boot, full init on cold boot** — recommended. Modem doesn't appear until after BIOS whitelist checks complete, bypassing laptop WWAN whitelists |
| `3` | Fast enum on both warm and cold boot |

```
AT!CUSTOM="FASTENUMEN",2    # Recommended: bypass BIOS whitelist on warm boot
```

---

## 8. Diagnostic & Status

### `AT!GSTATUS?`

Comprehensive modem status report. The single most useful diagnostic command.

```
!GSTATUS:
Current Time:  22016        Temperature: 40
Reset Counter: 1            Mode:        ONLINE
System mode:   LTE          PS state:    Attached
LTE band:      B2           LTE bw:      20 MHz
LTE Rx chan:   800          LTE Tx chan: 18800
LTE CA state:  ACTIVE       LTE Scell band:B12
LTE Scell bw:10 MHz        LTE Scell chan:5110
EMM state:     Registered   Normal Service
RRC state:     RRC Connected
IMS reg state: No Srv

PCC RxM RSSI:  -45          RSRP (dBm):  -81
PCC RxD RSSI:  -44          RSRP (dBm):  -79
SCC RxM RSSI:  -52          RSRP (dBm):  -83
SCC RxD RSSI:  -51          RSRP (dBm):  -81
Tx Power:      15           TAC:         8C65 (35666)
RSRQ (dB):     -14.0        Cell ID:     0A14666A (169106666)
SINR (dB):     23.8
```

| Field | Description |
|-------|-------------|
| Temperature | Modem temp in Celsius |
| System mode | Current RAT (LTE, UMTS, etc.) |
| PS state | Packet-Switched state (Attached/Detached) |
| LTE band | Current serving band |
| LTE bw | Channel bandwidth (1.4/3/5/10/15/20 MHz) |
| LTE CA state | Carrier Aggregation (ACTIVE/INACTIVE) |
| LTE Scell | Secondary cell info (when CA active) |
| EMM state | EPS Mobility Management state |
| RRC state | Radio Resource Control state |
| RSSI | Received Signal Strength Indicator (dBm) |
| RSRP | Reference Signal Received Power (dBm) — primary signal metric |
| RSRQ | Reference Signal Received Quality (dB) |
| SINR | Signal to Interference + Noise Ratio (dB) |
| TAC | Tracking Area Code |
| Cell ID | Serving cell identifier |
| PCC/SCC | Primary/Secondary Component Carrier (CA) |
| RxM/RxD | Main/Diversity antenna paths |

### `AT!LTEINFO?`

Detailed LTE serving cell and neighbor cell information.

```
!LTEINFO:
Serving:   EARFCN MCC MNC   TAC      CID Bd D U SNR PCI  RSRQ   RSRP   RSSI RXLV
              800 310 410 35666 0A14666A  2 5 5  22 404 -14.1  -79.6  -45.5 --

IntraFreq:                                          PCI  RSRQ   RSRP   RSSI RXLV
                                                    404 -14.1  -79.6  -45.5 --
                                                    402 -20.0  -89.3  -57.6 --

InterFreq: EARFCN ThresholdLow ThresholdHi Priority PCI  RSRQ   RSRP   RSSI RXLV
             5110            0           0        0 272 -12.1  -80.9  -50.6   0
```

| Field | Description |
|-------|-------------|
| EARFCN | E-UTRA Absolute Radio Frequency Channel Number |
| MCC/MNC | Mobile Country Code / Mobile Network Code |
| TAC | Tracking Area Code |
| CID | Cell ID |
| Bd | Band number |
| D/U | Downlink/Uplink category |
| SNR | Signal to Noise Ratio |
| PCI | Physical Cell ID |
| IntraFreq | Neighbor cells on same frequency |
| InterFreq | Neighbor cells on different frequencies |

### `AT!LTECA?` / `AT!LTECA=<0|1>`

Query supported Carrier Aggregation band combinations or enable/disable CA.

```
Hardware:
LTEB1: B8,
LTEB2: B2, B5, B12, B13, B29,
LTEB3: B7, B20,
LTEB4: B4, B5, B12, B13, B29,
...
```

```
AT!LTECA=0    # Disable carrier aggregation
AT!LTECA=1    # Enable carrier aggregation
```

> `AT!LTECA?` is not supported on firmware 02.38.00.00 — requires AT!OPENLOCK or downgrade to 02.33.03.00.

---

## 9. GPS

### `AT!GPSFIX=<mode>,<timeout>,<accuracy>`

Request a GPS fix.

| Parameter | Description |
|-----------|-------------|
| mode | `1` = standalone (no assistance) |
| timeout | Seconds to wait for fix |
| accuracy | Desired accuracy in meters |

```
AT!GPSFIX=1,30,10    # Standalone fix, 30s timeout, 10m accuracy
AT!GPSFIX=?          # List valid parameter ranges
```

### `AT!GPSLOC?`

Query current GPS location (after a fix is obtained).

### `AT!GPSSTATUS?`

Query GPS fix status and satellite info.

---

## 10. Standard 3GPP AT Commands

### `AT+CFUN=<mode>`

Set phone functionality mode.

| Value | Mode |
|-------|------|
| `0` | Minimum functionality (radio off) |
| `1` | Full functionality (radio on) |

```
AT+CFUN=0    # Power down radio
AT+CFUN=1    # Power on radio
```

### `AT+CGDCONT?` / `AT+CGDCONT=<cid>,"<type>","<apn>"`

Query or set PDP context definitions (APN settings).

```
AT+CGDCONT=1,"IP","internet"        # Set APN for context 1
AT+CGDCONT=1,"IPV4V6","broadband"   # Set dual-stack APN
```

### `AT+CPIN?`

Query SIM card status.

- `+CPIN: READY` — SIM present and unlocked
- `+CPIN: SIM PIN` — SIM requires PIN entry

### `AT+CIMI`

Query the IMSI (International Mobile Subscriber Identity) from the SIM card.

### `AT+CGSN`

Query the IMEI (same as the IMEI field from ATI).

### `AT+CSQ`

Query signal quality.

```
+CSQ: 25,99
```

First value is RSSI (0-31, 99=unknown), second is BER (0-7, 99=unknown).

### `AT+COPS?` / `AT+COPS=?`

Query current operator or scan for available networks.

### `AT+CSCA?`

Query SMS Service Centre Address.

### `AT+CMGF=<0|1>`

Set SMS format: `0` = PDU mode, `1` = text mode.

### `AT+CMGS="<number>"`

Send SMS message. After this command, enter the message text and terminate with Ctrl-Z.

---

## 11. Factory Reset

### `AT!RMARESET=1`

Factory reset to Lenovo / Sierra Wireless defaults. Clears all custom settings (VID/PID, bands, FASTENUMEN, PCOFFEN, etc.) and restores original OEM configuration.

### `AT!NVRESTORE=0`

Factory reset to Dell defaults. Similar to `AT!RMARESET` but restores Dell-specific NVM values.

---

## 12. Advanced / Engineering

### `AT!ENTERCND="A710"`

Unlock engineering command mode. Password `A710` is universal across EM74xx/MC74xx. Required before any `AT!` set commands.

### `AT!OPENLOCK`

Query the challenge string for OEM-level unlock. Returns a random challenge that must be responded to with a computed hash. Required for some commands on newer firmware (e.g., `AT!LTECA?` on fw 02.38.00.00).

### `AT!BOOTHOLD`

Enter bootloader hold mode for firmware download. Used by `qmi-firmware-update` during the flash process.

> Not implemented on very early firmware versions (SWI9X30C_00.08.02.00 found on engineering samples).

---

## 13. AirVantage OTA Updates

### `AT+WDSC=3,<minutes>`

Set AirVantage FOTA check-in interval in minutes.

### `AT+WDSS=1,1`

Send an immediate AirVantage heartbeat / check-in.

---

## Appendix: Known VID/PID Table

| Brand | VID | APP PID | BOOT PID | Model |
|-------|-----|---------|----------|-------|
| Sierra Wireless | `1199` | `9071` | `9070` | EM7455 (generic) |
| Lenovo | `1199` | `9079` | `9078` | EM7455 |
| Dell | `413C` | `81B6` | `81B5` | DW5811e |

## Appendix: Recommended Configuration

For most laptop installs, the recommended post-flash settings are:

```
AT!ENTERCND="A710"
AT!USBCOMP=1,1,0000100D      # MBIM mode
AT!USBVID=1199               # Generic Sierra VID
AT!USBPID=9071,9070          # Generic Sierra PIDs
AT!USBPRODUCT="EM7455"       # Generic product string
AT!SELRAT=06                  # LTE only
AT!BAND=00                    # All bands (or 09 for LTE-only bands)
AT!CUSTOM="FASTENUMEN",2     # Skip warm boot enumeration (bypass whitelist)
AT!PCOFFEN=2                  # Ignore W_DISABLE pin
AT!USBSPEED=0                 # Force USB 2.0
AT!RESET                      # Apply and reboot
```
