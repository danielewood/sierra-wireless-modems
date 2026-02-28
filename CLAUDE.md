# swtool — CLAUDE.md

## 0 — Project Overview

Go module: `github.com/danielewood/sierra-wireless-modems/swtool`
Go version: 1.26+
Pure Go build — no CGO required. Single static binary.

Sierra Wireless EM7455/MC7455/EM7565 modem management tool. Detects modems via sysfs, communicates via serial AT commands, downloads firmware from Sierra Wireless, flashes via qmi-firmware-update, and configures modem identity/bands/USB mode.

### External runtime dependencies

- `libqmi-utils` — provides `qmicli` and `qmi-firmware-update` (shell-out, to be replaced with pure Go QMI later)

### Quick reference

```sh
go build -o swtool ./swtool/                 # Build
go test ./swtool/...                          # Test
go vet ./swtool/...                           # Static analysis
sudo ./swtool info                            # Read modem settings (safe)
sudo ./swtool at "ATI"                        # Send AT command
sudo ./swtool flash                           # Full flash workflow
```

---

## 1 — Before Coding

- Ask clarifying questions for ambiguous requirements.
- Draft and confirm an approach before writing code.
- When >2 approaches exist, list pros/cons and rationale.

---

## 2 — Package Structure

```text
swtool/
  main.go                       # Entry point, cobra root init
  cmd/                          # Cobra subcommands (flash, info, download, configure, at, completion)
  modem/                        # Modem detection (sysfs), serial I/O, AT commands
  qmi/                          # Shell-out wrappers for qmicli and qmi-firmware-update
  firmware/                     # Firmware download, extraction, PRI ID parsing
  internal/sysutil/             # Root check, ModemManager, command runner
  internal/log/                 # slog wrapper with colored output
```

---

## 3 — Dependencies

- Prefer stdlib. The only external deps should be: `spf13/cobra`, `go.bug.st/serial`, `PuerkitoBio/goquery`.
- No test frameworks beyond stdlib `testing`.

---

## 4 — Code Style

### Go version

Target Go 1.26. Use modern stdlib features: `slices`, `maps`, `log/slog`, `min`/`max` builtins, range-over-int.

### Formatting and imports

- `gofmt`, `go vet`, `goimports` before committing.
- Two import groups: stdlib, then third-party. Alphabetical within each.

### Naming

- Avoid stutter: `modem.Device` not `modem.ModemDevice`.
- Exported functions: doc comment required.
- Error variables: `errFoo` (unexported), `ErrFoo` (exported).
- Test helpers: always call `t.Helper()`.
- Use input structs for functions receiving more than 2 arguments. Context is always a separate first param.

### Philosophy

- Boring and readable over clever and terse.
- No premature abstractions.
- Consistency with existing patterns trumps personal preference.
- DRY: extract helpers when logic repeats.

---

## 5 — Errors

- Wrap with `%w` and context: `fmt.Errorf("detecting modem: %w", err)`.
- Use `errors.Is`/`errors.As` for control flow; no string matching.
- Error strings are lowercase, no trailing punctuation.
- Never silently ignore errors.
- Fail fast — return errors immediately.
- Define sentinel errors in the package that produces them.

---

## 6 — Hardware Interaction Patterns

### Serial communication

- Always `defer port.Close()` immediately after opening.
- Set read timeouts. Never block indefinitely on serial reads.
- Log all AT commands sent and responses received at debug level.
- Validate responses: check for `OK` vs `ERROR` terminator.

### USB device detection

- Use sysfs (`/sys/bus/usb/devices/`) not lsusb/dmesg parsing.
- Polling loops must have timeouts. Never spin forever.
- Re-detect after every modem reset — device paths can change.

### External tool shell-out

- Always check binary exists in PATH before attempting to run.
- Capture both stdout and stderr.
- Log the full command at debug level before execution.
- Wrap exit errors with the stderr output for diagnostics.

---

## 7 — Testing

- Table-driven tests with descriptive subtest names.
- Run `-race` in CI.
- Tests use stdlib `testing` only.
- Mark safe tests with `t.Parallel()`.

### Test categories

- **Unit tests**: AT command parsing, PRI ID extraction, sysfs path logic. No hardware needed.
- **Integration tests**: Build-tagged `//go:build integration`. Require attached modem. Not run in CI.

### What to test

- Test this project's logic, not upstream behavior.
- One parametric test over N inputs, not N copy-paste tests.
- Test behavior through public API, not unexported helpers.

---

## 8 — CLI Output

- Stdout is for data (modem info, JSON output, AT responses). Stderr is for progress/status.
- `--json` flag outputs structured JSON to stdout.
- `--verbose` enables debug logging to stderr.
- `--quiet` suppresses non-essential stderr output.
- Exit codes: `0` = success, `1` = error.
- Colored output: cyan for step headers (matching original script style), red for errors.

---

## 9 — Logging

- `log/slog` exclusively. Never `log` or `fmt.Print` for diagnostics.
- Structured logging with consistent keys.
- Debug level for AT commands, external tool invocations, sysfs reads.
- Info level for user-facing step progress.
- Error level for failures.

---

## 10 — Git

- Commit messages explain "why", not "what".
- No direct pushes to main without review.
- `go vet`, `go test`, and `go build` must pass before committing.
