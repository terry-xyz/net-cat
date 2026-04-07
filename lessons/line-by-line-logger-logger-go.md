# line-by-line: logger/logger.go

## File Purpose

Thread-safe append-only daily logger with nil safety and file rollover.

## Why This File Matters

The server depends on logs for durable recovery. If log format or rollover semantics drift, restart behavior changes.

## Dependencies In and Out

- Inbound: `main.go`, `server/room.go`, `server/server.go`, `server/history.go`.
- Outbound: filesystem, `models.Message`.

## Ordered Walkthrough

- `logger/logger.go:14-20`: `Logger` holds a mutex, target directory, current open file, current date key, and a `closed` bit that prevents reopening after shutdown.
- `logger/logger.go:24-29`: `New` eagerly creates the directory. Failing here is the only hard logger construction error.
- `logger/logger.go:33-54`: `Log` is nil-safe, lock-protected, and date-routed. It refuses writes after `Close`, lazily opens the correct date file, formats one parseable line, and downgrades write/open failures to stderr warnings.
- `logger/logger.go:57-70`: `FilePath` and `Dir` are tiny helpers used by recovery and tests; both are nil-safe.
- `logger/logger.go:73-85`: `Close` flips `closed`, closes the current file if present, and prevents future implicit reopen.
- `logger/logger.go:89-92`: exported `FormatDate` is just a wrapper to keep date formatting shared without exposing internal file state.
- `logger/logger.go:93-108`: `ensureFile` implements day-based file switching. It closes the previous file when the date changes, opens or creates the new file in append mode, and updates the in-memory file/date pair.
- `logger/logger.go:111-113`: internal date formatting is fixed-width `YYYY-MM-DD`.

## Key Takeaways

- Logger errors are deliberately non-fatal.
- Recovery depends on `FormatLogLine` output staying parseable.
- `closed` prevents late goroutines from reopening files during shutdown.

## Audit Notes

- If you change file permissions, naming, or day switching, inspect recovery and day-boundary tests immediately.
