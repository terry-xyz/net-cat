# line-by-line: server/admins.go

## File Purpose

Persisted admin-list management.

## Ordered Walkthrough

- `server/admins.go:14-43`: `LoadAdmins` opens `admins.json`, tolerates missing files, downgrades corrupt JSON to a warning, and repopulates `s.admins`.
- `server/admins.go:44-85`: `SaveAdmins` snapshots the admin name set, insertion-sorts for deterministic output, marshals JSON, writes to a temp file, and renames atomically.
- `server/admins.go:86-117`: lookup and mutation helpers (`IsKnownAdmin`, `AddAdmin`, `RemoveAdmin`, `RenameAdmin`) keep the in-memory map updated and then persist.

## Key Takeaways

- Admin persistence is username-based, not connection-based.
- Atomic rename of the temp file is the crash-safety step.

## Audit Notes

- If usernames ever become case-insensitive or user IDs replace names, this file must be redesigned with migration in mind.
