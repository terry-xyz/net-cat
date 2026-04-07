# line-by-line: server/operator.go

## File Purpose

Implements the server-terminal control plane.

## Why This File Matters

It reuses the command vocabulary but not the user transport path. Operator behavior is similar to admin behavior but not identical.

## Ordered Walkthrough

- `server/operator.go:18-30`: `StartOperator` scans lines from an arbitrary reader, making the operator path testable and decoupled from `os.Stdin`.
- `server/operator.go:31-87`: `OperatorDispatch` trims input, requires slash-prefixed commands, validates command names against the shared registry, rejects inapplicable user-only commands, and routes to operator-specific handlers.
- `server/operator.go:88-95`: `operatorSend` writes to `OperatorOutput`, normally stdout.
- `server/operator.go:96-159`: operator `/list` groups output by room, lists idle times for active clients, and also shows queued users by host IP.
- `server/operator.go:160-167`: operator `/help` prints the full command registry without privilege filtering.
- `server/operator.go:168-210`: operator `/kick` kicks either an active username or queued users identified by IP.
- `server/operator.go:211-290`: operator `/ban` mirrors the user `/ban` logic, but can also ban queued users by IP directly and has no need to exclude the operator from host-collateral matching.
- `server/operator.go:291-346`: mute/unmute are thin wrappers around the same state changes as user-admin moderation, but with `"Server"` as the actor.
- `server/operator.go:347-365`: announcements are logged into every room and broadcast globally.
- `server/operator.go:366-425`: promote/demote mirror admin persistence behavior with `"Server"` as the acting identity.
- `server/operator.go:426-433`: operator `/rooms` prints room counts without room-current markers because the operator is not a room participant.

## Key Takeaways

- Operator commands are not just elevated admin commands; they also operate on queued users and have no room identity.

## Audit Notes

- When adding a new command, explicitly decide whether it makes sense for the operator path, not just whether it exists in the shared registry.
