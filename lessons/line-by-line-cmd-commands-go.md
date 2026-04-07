# line-by-line: cmd/commands.go

## File Purpose

Canonical command registry and parser.

## Why This File Matters

Both client-side `/help` and operator `/help` rely on this file. Privilege checks in `server/commands.go` and command applicability in `server/operator.go` both assume this registry is authoritative.

## Dependencies In and Out

- Inbound: `server/commands.go`, `server/operator.go`.
- Outbound: only `strings`.

## Ordered Walkthrough

- `cmd/commands.go:6-12`: `PrivilegeLevel` encodes the three authority tiers: user, admin, operator.
- `cmd/commands.go:15-20`: `CommandDef` stores the minimum privilege plus display metadata.
- `cmd/commands.go:23-39`: `Commands` is the registry. This is the declarative list of all supported commands and their help text.
- `cmd/commands.go:42-46`: `CommandOrder` fixes help output order independently of map iteration order.
- `cmd/commands.go:50-63`: `ParseCommand` treats only slash-prefixed input as a command, preserves case, accepts a lone slash as a command-shaped but invalid name, and trims outer whitespace from the argument tail while preserving internal spacing.
- `cmd/commands.go:66-74`: `GetPrivilegeLevel` maps booleans to the highest effective privilege, with operator dominating admin.

## Key Takeaways

- Parsing and registry declaration are intentionally separated from execution.
- Case sensitivity is part of the command contract.

## Audit Notes

- If you add a command, update both `Commands` and `CommandOrder`, then inspect both user and operator dispatchers.
