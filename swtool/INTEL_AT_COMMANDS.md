# Intel XMM AT Command Reference

Reference for Intel XMM 7160 (Infineon)-based modems such as the Sierra Wireless EM7345.
These modems use the Intel/Infineon AT command set, **not** the Sierra Wireless `AT!` command set
used by Qualcomm-based modems (EM7455, MC7455, EM7565).

## Key Differences from Qualcomm-Based Sierra Modems

| Feature | EM7455 (Qualcomm) | EM7345 (Intel XMM 7160) |
|---|---|---|
| USB VID:PID | `1199:9071` | `1199:a001` |
| AT port | `/dev/ttyUSB2` (qcserial, interface 03) | `/dev/ttyACM0` (cdc_acm, interface 02) |
| Data interface | QMI or MBIM | MBIM only |
| AT command prefix | `AT!` (Sierra proprietary) | `AT+X` (Intel proprietary) |
| Band selection | `AT!BAND=` | `AT+XACT=` |
| RAT selection | `AT!SELRAT=` | `AT+XACT=` (combined) |
| Firmware info | `AT!PRIID?` / `AT!IMAGE?` | `AT+XGENDATA` |
| USB composition | `AT!USBCOMP=` | N/A (fixed by firmware) |
| Signal quality | `AT!GSTATUS?` | `AT+XCESQ?` |
| Engineering password | `AT!ENTERCND="A710"` | N/A |
| Reboot | `AT!RESET` | `AT+CFUN=16` |

## Sources

- [ModemManager XMM plugin](https://github.com/linux-mobile-broadband/ModemManager/tree/main/src/plugins/xmm) (mm-modem-helpers-xmm.c) — canonical open-source parser
- [Telit xN930 AT Command Reference Guide](http://www.iot.com.tr/uploads/pdf/Telit_xN930_AT_Commands_Reference_Guide_r1.pdf) (xN930 is an XMM 7160 module)
- [zukota.com EM7345 AT commands](https://zukota.com/posts/sierra-wireless-em7345-at-commands/)
- Hardware testing against Sierra Wireless EM7345 (FIH7160, firmware V1.2 WW 01.1644.00)

---

## Identification

### AT+CGMI — Manufacturer
```
AT+CGMI
Sierra Wireless Inc.
OK
```

### AT+CGMM — Model
```
AT+CGMM
Sierra Wireless EM7345 4G LTE
OK
```

### AT+CGMR — Firmware Revision
```
AT+CGMR
V1.1,11
OK
```

### AT+CGSN — IMEI
```
AT+CGSN
013937006598898
OK
```

### ATI — Device Information
Returns the IMEI followed by a hardware revision indicator.
```
ATI
0139370065988911
OK
```

---

## Firmware

### AT+XGENDATA — Firmware Build Information
Returns the full build string including platform, variant, and build date.
```
AT+XGENDATA
+XGENDATA: "    FIH7160_XMM7160_V1.2_MBIM_GNSS_NAND_REV_4.5 2016-Oct-20 09:18:18
*FIH7160_V1.2_WW_01.1644.00_TS*"
OK
```

Format: `<platform>_<chipset>_<fwver>_<features> <date> *<carrier_variant>*`

Known firmware versions:
- `FIH7160_V1.1_WW_01.1410.13` — original factory (has LTE bugs)
- `FIH7160_V1.2_WW_01.1415.07` — early v1.2
- `FIH7160_V1.2_WW_01.1522.02` — adds AT+XCIQ control
- `FIH7160_V1.2_WW_01.1644.00_TS` — latest known

---

## Power & Control

### AT+CFUN — Functionality Control
Query or set the modem's power/radio state.

```
AT+CFUN?
+CFUN: 1,0          # <fun>,<rst>
OK
```

| Value | Meaning |
|-------|---------|
| 0 | Minimum functionality (radio off) |
| 1 | Full functionality (radio on) |
| 4 | Disable transmit and receive (flight mode) |
| 16 | **Reboot modem** (equivalent to AT!RESET on Qualcomm) |

### AT+CPWROFF — Power Off
Shuts down the modem gracefully.

---

## Network Technology & Band Selection

### AT+XACT — Access Technology and Band Configuration

Combined RAT preference and band selection in a single command. This replaces both
`AT!SELRAT` and `AT!BAND` from Qualcomm modems.

**Query current configuration:**
```
AT+XACT?
+XACT: 6,2,1,900,1800,1900,850,1,2,4,5,8,101,102,103,104,105,107,108,113,117,118,119,120
```

**Query supported values:**
```
AT+XACT=?
+XACT: (0-6),(0-2),0,900,1800,1900,850,1,2,4,5,8,101,102,103,104,105,107,108,113,117,118,119,120
```

**Set syntax:**
```
AT+XACT=<mode>[,<pref1>[,<pref2>[,<band1>[,<band2>[,...]]]]]
```

#### Mode Values (first parameter)

| Value | Technologies | Description |
|-------|-------------|-------------|
| 0 | 2G only | GSM/EDGE |
| 1 | 3G only | UMTS/HSPA |
| 2 | 4G only | LTE |
| 3 | 2G + 3G | GSM + UMTS |
| 4 | 3G + 4G | UMTS + LTE |
| 5 | 2G + 4G | GSM + LTE |
| 6 | 2G + 3G + 4G | All technologies (default) |

#### Preference Values (second parameter)

When multiple technologies are enabled, sets fallback priority:

| Value | Meaning |
|-------|---------|
| 0 | No preference |
| 1 | 3G preferred |
| 2 | 4G/LTE preferred |

#### Band Numbering Scheme

Bands are identified by numeric codes in a flat list after the preference fields:

| Range | Technology | Examples |
|-------|-----------|----------|
| > 300 | 2G (GSM) | 850, 900, 1800, 1900 (literal MHz) |
| 1–99 | 3G (UMTS) | 1=Band I, 2=Band II, 4=Band IV, 5=Band V, 8=Band VIII |
| 100–299 | 4G (LTE) | 101=B1, 102=B2, ..., 120=B20 (100 + band number) |

#### EM7345 Supported Bands

From `AT+XACT=?`:

| Tech | Bands | Frequencies |
|------|-------|-------------|
| 2G | 850, 900, 1800, 1900 | GSM 850/900/1800/1900 MHz |
| 3G | 1, 2, 4, 5, 8 | UMTS I/II/IV/V/VIII |
| 4G | 101–105, 107, 108, 113, 117–120 | LTE B1–B5, B7, B8, B13, B17–B20 |

#### Examples

Lock to LTE only, band 20:
```
AT+XACT=2,,,120
```

LTE only, bands 7 and 20:
```
AT+XACT=2,,,107,120
```

3G only, band I:
```
AT+XACT=1,,,1
```

All technologies, LTE preferred (factory default):
```
AT+XACT=6,2,1,900,1800,1900,850,1,2,4,5,8,101,102,103,104,105,107,108,113,117,118,119,120
```

---

## Network Registration

### AT+CREG — CS Network Registration
```
AT+CREG?
+CREG: 0,0          # <n>,<stat>
```

### AT+CEREG — EPS (LTE) Network Registration
```
AT+CEREG?
+CEREG: 0,0
```

### AT+CGREG — GPRS Network Registration
```
AT+CGREG?
+CGREG: 0,4
```

### AT+XREG — Extended Registration (Intel)
Reports the current access technology and band in use.
```
AT+XREG?
+XREG: <n>,<stat>[,<band_info>,<extra>]
```

**stat values:** Same as 3GPP (0=not registered, 1=home, 2=searching, 3=denied, 4=unknown, 5=roaming)

When registered, includes band info:
```
+XREG: 0,8,BAND_LTE_20,0    # Registered on LTE band 20
+XREG: 0,8,BAND_UMTS_I,0    # Registered on UMTS band I
```

### AT+COPS — Operator Selection
```
AT+COPS?
+COPS: 0             # 0 = automatic mode
```

### AT+XCOPS — Extended Operator Info (Intel)
Returns operator name in multiple formats. May return ERROR if not registered.

---

## Signal Quality

### AT+CSQ — Signal Quality (Standard)
```
AT+CSQ
+CSQ: 99,99          # <rssi>,<ber>  (99 = not available)
```

### AT+XCESQ — Extended Signal Quality (Intel)
Reports signal metrics for the currently active technology. This is the primary signal
quality command for Intel XMM modems.

```
AT+XCESQ?
+XCESQ: <n>,<rxlev>,<ber>,<rscp>,<ecn0>,<rsrq>,<rsrp>,<rssnr>
```

| Field | Range | Technology | Meaning | Unavailable |
|-------|-------|-----------|---------|-------------|
| n | — | — | Ignored | — |
| rxlev | 0–63 | 2G (GSM) | Receive level (maps to RSSI) | 99 |
| ber | 0–7 | 2G (GSM) | Bit error rate | 99 |
| rscp | 0–96 | 3G (UMTS) | Received signal code power | 255 |
| ecn0 | 0–49 | 3G (UMTS) | Ec/N0 (chip energy / noise) | 255 |
| rsrq | 0–34 | 4G (LTE) | Reference signal receive quality | 255 |
| rsrp | 0–97 | 4G (LTE) | Reference signal receive power | 255 |
| rssnr | -100–100 | 4G (LTE) | Signal-to-noise ratio (0.5 dB units) | 255 |

**Value conversions** (per 3GPP TS 27.007):
- rxlev → RSSI: `(rxlev - 110)` dBm approximately
- rscp → RSCP: `(rscp - 121)` dBm
- ecn0 → Ec/Io: `(ecn0 / 2 - 24.5)` dB
- rsrq → RSRQ: `(rsrq / 2 - 19.5)` dB
- rsrp → RSRP: `(rsrp - 141)` dBm
- rssnr → SNR: `(rssnr / 2)` dB

Example (registered on LTE):
```
+XCESQ: 0,99,99,255,255,24,51,18
```
Only the LTE fields (rsrq=24, rsrp=51, rssnr=18) have values; 2G/3G fields show unavailable.

---

## Error Reporting

### AT+XLOG — Exception/Crash Log
Read or clear the modem's internal error/crash log.

```
AT+XLOG=0    # Read crash log (can be very long)
AT+XLOG=2    # Clear crash log
```

Trap classes:
- `0xDDDD` — Software-generated trap
- `0xAAAA` — Hardware data abort
- `0xEEEE` — Software exception

### AT+XEER — Extended Error Report (Intel)
```
AT+XEER
+XEER: 0,"No extended error to report "
```

### AT+CEER — Call/Bearer Error Report
```
AT+CEER
+CEER: "No report available"
```

### AT+NEER — Network Error Report
```
AT+NEER
+NEER: No Extended Error available
```

### AT+XSYSERR — System Error
```
AT+XSYSERR?     # Query system error status
```

---

## SIM Card

### AT+CPIN — PIN Status
```
AT+CPIN?
+CPIN: READY        # SIM recognized, no PIN needed
ERROR               # No SIM / SIM not detected
```

### AT+CIMI — Read IMSI
Returns the International Mobile Subscriber Identity.

### AT+CCID — Read ICCID
Returns the SIM card ICCID.

### AT+CLCK — Facility Lock (SIM Lock Check)
```
AT+CLCK="PN",2      # Check network lock: 0=unlocked, 1=locked
```

---

## HSPA Configuration

### AT+XHSDUPA — HSPA Category
```
AT+XHSDUPA?
+XHSDUPA: 1, 24, 1, 6
```
Fields: `<hsdpa_enable>, <hsdpa_cat>, <hsupa_enable>, <hsupa_cat>`
- HSDPA Cat 24 = up to 42 Mbps DL
- HSUPA Cat 6 = up to 5.76 Mbps UL

---

## LTE Capabilities

### AT+XLCAPS — LTE Capability Summary
```
AT+XLCAPS?
+XLCAPS: 1,1,1,1,0,0,0,0,0,1,16,16,16,1
```

### AT+WS46 — Wireless Data Service
```
AT+WS46?
+WS46: 25           # 25 = 3GPP + LTE
```

---

## Battery

### AT+CBC — Battery Charge
```
AT+CBC
+CBC: 2,0            # <bcs>,<bcl>  (2 = no battery, 0 = charge level)
```

---

## GPS / GNSS

The EM7345 has a built-in GNSS receiver. Key commands:

### AT+XLCSLSR — Location Services Request
Configure and start a location fix.

```
AT+XLCSLSR=?
+XLCSLSR:(0-2),(0-3), ,(0,1), ,(0,1),(0-7200),(0-255),(0-1),(0-2),(1-256),(0,1)
```

Parameters:
1. Transport protocol: 1=SUPL, 2=None
2. Position mode: 2=MS-assisted/based, 3=Standalone
3–6. Client/MLC config (usually empty)
7. Interval (0–7200 seconds)
8. Service type ID
9. Pseudonym indicator
10. Response type: 1=NMEA
11. NMEA mask (bitmask for sentence types)
12. GNSS type: 0=GPS+GLONASS

### AT+XLCSSLP — SUPL Server
```
AT+XLCSSLP?
+XLCSSLP: <type>,<address>,<port>
```
Type: 0=IPv4, 1=FQDN

### AT+XLGNMEA — NMEA Output
### AT+XLGINFO — GNSS Information
### AT+XLGTEST — GNSS Test Mode

---

## Data Connection

### AT+CGDCONT — PDP Context Definition
```
AT+CGDCONT?          # List defined PDP contexts
AT+CGDCONT=1,"IPV4V6","apn.name"
```

### AT+CGATT — PS Attach/Detach
```
AT+CGATT?
+CGATT: 0            # 0=detached, 1=attached
```

### AT+CGACT — PDP Context Activation
```
AT+CGACT=1,1         # Activate context 1
```

### AT+XDNS — DNS Server Query
```
AT+XDNS?             # Query DNS servers for active contexts
```

### AT+XDATACHANNEL — Data Channel Configuration

---

## Messaging (SMS)

Standard 3GPP SMS commands are supported:
- `AT+CMGF` — Message format (0=PDU, 1=text)
- `AT+CMGL` — List messages
- `AT+CMGR` — Read message
- `AT+CMGS` — Send message
- `AT+CMGD` — Delete message
- `AT+CSCA` — Service center address
- `AT+CNMI` — New message indication

---

## Complete AT+CLAC Command List

Full output from `AT+CLAC` on EM7345 (firmware V1.2 WW 01.1644.00):

<details>
<summary>Click to expand (200+ commands)</summary>

```
ATS, ATD, ATA, ATO, ATE, ATH, ATV, ATZ, ATl, ATm, AT&K, ATI, ATQ, ATX,
AT&F, AT&D, AT&C, AT\Q,
AT+CMER, AT+CGSMS, AT+CMGD, AT+CMGF, AT+CMGL, AT+CMGR, AT+CMGS,
AT+CMGW, AT+CMMS, AT+CMSS, AT+CNMA, AT+CNMI, AT+CPMS, AT+CSCA,
AT+CSCB, AT+CSMS, AT+CNEM, AT+XSMSFDN, AT+XCSSMS, AT+XSMS, AT+XTESM,
AT+XWAPNMI, AT+CSAS, AT+CRES, AT+CSDH, AT+CMGC, AT+CSMP,
AT+CGREG, AT+COPN, AT+COPS, AT+CREG, AT+CSQ, AT+XCOPS, AT+XCSPAGING,
AT+XEONS, AT+XREG, AT+XAACOPS, AT+XACT, AT+WS46, AT+CEREG, AT+CEMODE,
AT+XLRTA, AT+XRAT, AT+CPLS, AT+CPOL, AT+XHOMEZR, AT+CIREP, AT+XCSQ,
AT+XSYSERR, AT+XCSFB, AT+XDAMCFG, AT+XCMI, AT+XMCI,
AT+CHUP, AT+CMOD, AT+CMUT, AT+CTFR, AT+VTS, AT+XCALLSTAT,
AT+XSPEECHCONFIG, AT+XDIAG, AT+XEMC, AT+XDTMF, AT+XVTS, AT+CSTA,
AT+CVHU, AT+VTD, AT+CCWE, AT+CR, AT+CRC, AT+XPROGRESS, AT+XREDIAL,
AT+XLIN,
AT+CAOC, AT+CCFC, AT+CCWA, AT+CHLD, AT+CLCC, AT+CLCK, AT+CLIP, AT+CLIR,
AT+CNAP, AT+COLP, AT+COLR, AT+CPWD, AT+CSSN, AT+CUSD, AT+CCUG, AT+CBST,
AT+CEER,
AT+CGACT, AT+CGANS, AT+CGATT, AT+CGAUTO, AT+CGCLASS, AT+CGCMOD,
AT+CGDATA, AT+CGDCONT, AT+CGDSCONT, AT+CGEQMIN, AT+CGEQNEG, AT+CGEQREQ,
AT+CGEREP, AT+CGPADDR, AT+CGQMIN, AT+CGQREQ, AT+CGTFT, AT+XCGCLASS,
AT+XDNS, AT+XGAUTH, AT+CSCON, AT+XMULTISLOT, AT+XGCNTRD, AT+XDATASTAT,
AT+XGCNTSET, AT+FCLASS, AT+CRLP, AT+CGEQOS, AT+CGEQOSRDP,
AT+CGTFTRDP, AT+CGCONTRDP, AT+CGSCONTRDP, AT+XPCO, AT+XNVMPLMN,
AT+XNVMMCC,
AT+CBC, AT+CCID, AT+CCLK, AT+CFUN, AT+CGMI, AT+GMI, AT+CGMM, AT+GMM,
AT+CGMR, AT+GMR, AT+CGSN, AT+GSN, AT+CIMI, AT+CMEE, AT+CMUX, AT+CNUM,
AT+CPIN, AT+CPWROFF, AT+CRSM, AT+CSCS, AT+CSIM, AT+CSVM, AT+CTZR,
AT+CTZU, AT+IPR,
AT+XCTMS, AT+XGENDATA, AT+XPINCNT, AT+XLOG, AT+CMMIVT, AT+XCESQ,
AT+XMER, AT+XSIMSTATE, AT+XSYSCHANGEIND, AT+CIND, AT+CSUS, AT+TRACE,
AT+XL1SET, AT+XSIO, AT+XDLCTEST, AT+XPOW, AT+XCEER, AT+XEER,
AT+XTRACECONFIG, AT+XMUX, AT+XFDOR, AT+XFDORT, AT+XTDEV, AT+XCFC,
AT+XEMN, AT+CSSAC, AT+XCONFIG, AT+XAPP, AT+XHSDUPA, AT+XCAP,
AT+XFDSLEEP,
AT+CGED, AT+XCELLINFO, AT+XCGEDPAGE, AT+XMETRIC, AT+XNRTAPP,
AT+XNRTCWS,
AT+CAMM, AT+CACM, AT+XDATACHANNEL, AT+CPIN2, AT+CONNECTPORT, AT+CCHO,
AT+CCHC, AT+XCSP, AT+NEER, AT+CUAD, AT+CEAP, AT+CERP, AT+XUICC,
AT+XLEMA, AT+XAUTH, AT+FMR, AT+XSYSTRACE, AT+XDBGCONF, AT+XABBTRACE,
AT+CLAC, AT+CPUC, AT+CLAN, AT+CGLA, AT+CRLA, AT+CPAS, AT+SETLITRACE,
AT+XSVM, AT+XNOTIFYDUNSTATUS, AT+XRXDIV, AT+XRXDIV3GRAB,
AT+XMAGETKEY, AT+XMAGETBLOCK, AT+XTSM, AT+XTAMR, AT+XADPCLKFREQINFO,
AT+XBCON, AT+XBDISC, AT+XBATR, AT+XBAPDU, AT+XBPWR, AT+XBCSTAT,
AT+XBCRDSTAT,
AT+XLGNMEA, AT+XLGTEST, AT+XLGINFO, AT%GPS, AT+XLCSLSR, AT+XLSRSTOP,
AT+XLTC, AT+XLOMR, AT+XLCSLRMT, AT+XLCSLRV, AT+XLCSLQOS, AT+XLICLS,
AT+XLCSVER, AT+XLCSSLP, AT+XLICLP, AT+XLCSLUI, AT+XLCSTER, AT+XLGASSIST,
AT+XLGNVRAM, AT+XLCSSHUTDOWN, AT%XLCSTEST, AT+XLCSSUPLVER,
AT+XLCSLSTR, AT+XLCSSUPLAPPID, AT+XLCSAETTA, AT+XLCSTTTPLR,
AT+XLCSTPLR, AT+XLCSSWITCH, AT+XLCSINIT, AT+XLCSSTOREAPN, AT+XLGTSR,
AT+CPOS, AT+XLSRSTOP, AT+CPOSR, AT+CMOLR, AT+CMTLR, AT+CMTLRA,
AT+XCPOSR, AT+XLCAPS, AT+CMOLRE,
AT+CPBR, AT+CPBS, AT+CPBW, AT+CPBF, AT+XCPBW, AT+XCPBR,
AT+XBIP, AT+STKPROF, AT+SATR, AT+SATE, AT+STKCTRLIND, AT+SATD,
AT+XATTMODE, AT+XSETCAUSE, AT+XSPEECHINFO, AT+XUCCI, AT@NVM,
AT+CGPIAF, AT+XSDT, AT+XCCINFO, AT+XIPCONFIG, AT+XSTRESSSIM,
AT+XSATK, AT*CNTI, AT+XCOLP, AT+XNITZINFO, AT+XCALLNBMMI,
AT+XRLCSET
```
</details>

---

## Firmware Flashing

The EM7345 uses Intel's FlashTool (FlashTool_E2.exe) with `.fls` firmware files, **not**
`qmi-firmware-update`. The flashing process is fundamentally different from Qualcomm modems:

- Firmware files are in FLS/FLZ format (not CWE/NVU)
- Bootloader mode shows USB ID `8087:0716` (Intel)
- Uses Intel MBIM Toolkit / Firmware Updater, not QDL protocol
- Firmware stored at `C:\ProgramData\Intel\MBIM Toolkit\FirmwareDatabase\PreInstalled` (Windows)

**Warning:** The `AT+XCIQ=0` command on firmware versions before 1522.02 will brick the modem.
Use `AT+XLOG=0` to check for accumulated crash logs.

---

## Differences Summary for swtool

Commands that **work** on EM7345:
- Standard 3GPP: `AT+CGMI`, `AT+CGMM`, `AT+CGMR`, `AT+CGSN`, `AT+CSQ`, `AT+COPS`, `AT+CREG`, `AT+CEREG`, `AT+CGREG`, `AT+CFUN`, `AT+CPIN`, `AT+CIMI`, `AT+CCID`
- Intel-specific: `AT+XACT`, `AT+XCESQ`, `AT+XREG`, `AT+XGENDATA`, `AT+XLOG`, `AT+XEER`, `AT+XHSDUPA`, `AT+XLCAPS`

Commands that **do NOT work** on EM7345 (Qualcomm/Sierra only):
- `AT!ENTERCND`, `AT!USBCOMP`, `AT!USBVID`, `AT!USBPID`, `AT!PRIID`, `AT!IMPREF`, `AT!IMAGE`, `AT!BAND`, `AT!SELRAT`, `AT!GSTATUS`, `AT!CUSTOM`, `AT!RESET`, `AT!LTEINFO`, `AT!LTECA`, `AT!PCINFO`, `AT!PCOFFEN`
