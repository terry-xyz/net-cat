# line-by-line: server/history.go

## File Purpose

History reset, history append, combined-history compatibility view, and startup recovery from log files.

## Ordered Walkthrough

- `server/history.go:16-25`: `ClearHistory` zeroes room histories in place at midnight.
- `server/history.go:26-37`: `AddHistory` routes a message to its room or to `DefaultRoom` when the room field is empty.
- `server/history.go:38-52`: `GetHistory` flattens every room’s history into one slice for backward compatibility with earlier code and tests.
- `server/history.go:53-111`: `RecoverHistory` opens today’s log file from the configured logger, scans it line by line, skips corrupt lines with warnings, drops server-only events, assigns old log lines with no room tag to `DefaultRoom`, and appends recovered events directly into room histories.

## Audit Notes

- Recovery assumes the logger uses local date boundaries. If the project ever moves to UTC-based files, both logger and recovery need to change together.
