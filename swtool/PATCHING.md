# NVU Binary Patching

## NVU File Format

NVU (Non-Volatile User) files are Sierra Wireless carrier configuration packages.
They use the same CWE (Compact Wireless Equipment) container format as firmware
`.cwe` files — nested headers wrapping an NVUP delta payload.

### CWE Header (0x190 = 400 bytes, big-endian)

| Offset | Size | Field | Description |
|--------|------|-------|-------------|
| 0x000 | 256 | `reserved1` | Outer envelope (first header) or zeros (embedded) |
| 0x100 | 4 | `crc` | CRC32-MPEG2 of `reserved1` |
| 0x104 | 4 | `rev` | Header revision |
| 0x108 | 4 | `val` | CRC validity indicator (`0xFFFFFFFF` or `"GOOD"`) |
| 0x10C | 4 | `type` | ASCII: `SPKG`, `FILE`, or `NVUP` |
| 0x110 | 4 | `product` | ASCII: `9X30` for EM7455/MC7455 |
| 0x114 | 4 | `imgsize` | Size of data after this header |
| 0x118 | 4 | `imgcrc` | CRC32-MPEG2 of the image data |
| 0x11C | 84 | `version` | Null-terminated version string |
| 0x170 | 8 | `date` | Build date (MM/DD/YY) |
| 0x178 | 4 | `compat` | Backward compatibility field |
| 0x17C | 20 | `reserved2` | Padding |

### NVU Structure (nested)

```
[0x000] SPKG header     — outer package, version string has fw version + PRI rev
  [0x190] FILE header   — inner package descriptor (same version string)
    [0x320] FILE header — path: /swir/nvdelta/NVUP_GENERIC.010
      [0x4b0] NVUP header — version string has fw version + PRI rev
        [0x640] NVUP payload — NV item records
```

Each header's `imgsize` covers everything after it (child headers + payload).

### NVUP Payload

The payload contains NV item records with paths like:

- `/nv/item_files/modem/mmode/sms_only`
- `/nv/item_files/modem/lte/rrc/cap/diff_fdd_tdd_fgi_enable`
- `/nv/item_files/modem/qmi/uim/auth_security_restrictions`

And named configuration values:

- `PRI_CARRIER_PN` — PRI part number (e.g., `9904609`)
- `PRI_CARRIER_REV` — PRI revision (e.g., `0285` for rev 002.085)
- `SUPPORTED_PLMNS` — carrier PLMN whitelist

## CRC Algorithm

Both `crc` and `imgcrc` fields use **CRC32-MPEG2**:

- Polynomial: `0x04C11DB7`
- Initial value: `0xFFFFFFFF`
- Final XOR: `0x00000000` (none)

This is standard CRC32 without the final inversion. In Python:

```python
import zlib

def crc32_mpeg2(buf: bytes) -> int:
    return (zlib.crc32(buf) & 0xFFFFFFFF) ^ 0xFFFFFFFF
```

The `crc` field covers the 256-byte `reserved1` area of the same header.
The `imgcrc` field covers `imgsize` bytes starting immediately after the header.

### CRC Recomputation Order

Because headers are nested (outer `imgcrc` covers inner headers), CRCs must be
recomputed **inside-out**: NVUP first, then its parent FILE, then the next FILE,
then the outer SPKG.

## Patching to Change PRI Identity

When flashing firmware version X with an NVU built for version Y, the modem
detects the PRI version mismatch and `IMSWITCH` votes to keep the modem in
Low Power Mode (LPM). Patching the NVU's identity strings to match the target
firmware prevents this.

### What to Patch

All patches are same-length ASCII string replacements (no size changes):

1. **Version strings** in headers at 0x000, 0x190, and 0x4b0 (offset 0x11C
   within each header) — the `version` field contains the full descriptor like
   `9999999_9904609_SWI9X30C_02.30.01.01_00_GENERIC_002.045_001`

2. **`PRI_CARRIER_REV` NV value** in the NVUP payload — stored as a
   zero-padded ASCII string (e.g., `0245` for rev 002.045)

### Example: Patch Legacy NVU (02.30) to Identify as Current (02.39)

```python
import struct, zlib, shutil

def crc32_mpeg2(buf: bytes) -> int:
    return (zlib.crc32(buf) & 0xFFFFFFFF) ^ 0xFFFFFFFF

src = "SWI9X30C_02.30.01.01_GENERIC_002.045_001.nvu"
dst = "SWI9X30C_02.30.01.01_GENERIC_002.045_001_patched_as_085.nvu"
shutil.copy2(src, dst)

with open(dst, "rb") as f:
    data = bytearray(f.read())

# Step 1: Patch strings
patches = [
    # Firmware version in 3 header version strings
    (0x0135, b"02.30.01.01", b"02.39.00.00"),
    (0x02C5, b"02.30.01.01", b"02.39.00.00"),
    (0x05E5, b"02.30.01.01", b"02.39.00.00"),
    # PRI revision in 3 header version strings
    (0x014C, b"002.045", b"002.085"),
    (0x02DC, b"002.045", b"002.085"),
    (0x05FC, b"002.045", b"002.085"),
    # PRI_CARRIER_REV NV value in NVUP payload
    (0x0B0B, b"0245", b"0285"),
]

for offset, old, new in patches:
    assert data[offset : offset + len(old)] == old
    data[offset : offset + len(new)] = new

# Step 2: Recompute CRCs inside-out
headers = [
    (0x4B0, 0x0610),  # NVUP (innermost)
    (0x320, 0x07A0),  # FILE
    (0x190, 0x0930),  # FILE
    (0x000, 0x0AC0),  # SPKG (outermost)
]

for off, imgsize in headers:
    # reserved1 CRC
    struct.pack_into(">I", data, off + 0x100, crc32_mpeg2(bytes(data[off : off + 0x100])))
    # image data CRC
    struct.pack_into(">I", data, off + 0x118, crc32_mpeg2(bytes(data[off + 0x190 : off + 0x190 + imgsize])))

with open(dst, "wb") as f:
    f.write(data)
```

### Flashing

```bash
sudo qmi-firmware-update --update \
  --cdc-wdm /dev/cdc-wdm0 \
  --device-open-mbim -p \
  --carrier GENERIC \
  SWI9X30C_02.39.00.00.cwe \
  SWI9X30C_02.30.01.01_GENERIC_002.045_001_patched_as_085.nvu
```

## Why This Works

The modem's QDL bootloader validates CWE header CRCs during flash writes
(error 137 = CRC/signature failure). The host-side tool (`qmi-firmware-update`
from libqmi) does **not** validate CRCs — it reads headers for metadata only
and passes the raw bytes to the modem.

The modem checks the PRI identity after boot to decide whether the loaded NVU
matches the running firmware. A mismatch causes `IMSWITCH` to vote for LPM,
locking the modem in low-power mode. By patching only the identity fields
(version strings and `PRI_CARRIER_REV`), the modem accepts the NVU as matching
while the actual NV configuration (band masks, APNs, PLMN lists) remains from
the donor NVU.

## Comparison: Legacy vs Current NVU

| | Legacy (02.30.01.01) | Current (02.39.00.00) |
|---|---|---|
| PRI Revision | 002.045 | 002.085 |
| File size | 3,152 bytes | 23,855 bytes |
| NVUP payload | 1,152 bytes | 21,855 bytes |
| NV items | 8 | 10 |
| PLMN table | Wildcards only (~100 B) | Full global operator list (~20 KB) |
| Build date | 09/24/18 | 06/05/24 |

The two NV items added in the current NVU:

- `/nv/item_files/modem/lte/ML1/disable_single_rx_chain`
- `/nv/item_files/modem/lte/rrc/cap/diff_fdd_tdd_fgi_enable`

## Sources

- [libqmi CWE image parser](https://gitlab.freedesktop.org/mobile-broadband/libqmi/-/blob/main/src/qmi-firmware-update/qfu-image-cwe.c) — `QfuCweFileHeader` struct definition
- [QMI firmware update with libqmi](https://sigquit.wordpress.com/2016/12/09/qmi-firmware-update-with-libqmi/) — background on the flash process
