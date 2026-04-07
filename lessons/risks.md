# Net-Cat Risks

## Registry Coherence

- Invariant: every active username in `s.clients` must match the same username inside that client’s room map.
- Files: `server/clients.go:30`, `server/clients.go:89`, `server/room.go:56`.
- Risk: rename, disconnect, switch-room, kick, and ban all mutate registry state. Missing one map update causes stale lookups, ghost users, or failed room broadcasts.
- If you change `RegisterClient`, `RemoveClient`, `RenameClient`, or `JoinRoom`, inspect all four files above plus `server/commands.go`.

## Direct Connection Writes

- Invariant: writes should normally go through `Client.writeLoop()`.
- Files: `client/client.go:236`, `server/handler.go:51`, `server/handler.go:58`, `server/server.go:251`.
- Risk: onboarding rejection and heartbeat probes bypass the write loop for timing reasons. Adding more direct writes can scramble prompt redraws or block on `net.Pipe` in tests.

## Shutdown Coverage

- Invariant: shutdown must notify and close onboarding clients, queued clients, and active room members.
- Files: `server/server.go:25`, `server/server.go:100`, `server/clients.go:12`.
- Risk: code that only walks `s.clients` misses users in earlier lifecycle phases.

## NAT Collateral on Ban

- Invariant: ban logic operates on host IP, not logical username.
- Files: `server/moderation.go:11`, `server/commands.go:398`, `server/operator.go:211`.
- Risk: banning one client disconnects all clients sharing that IP and removes queued users from the same host. This is tested and intentional, but operationally severe.

## Queue Admission Edge Cases

- Invariant: each room queue is FIFO and emits queue-position updates when membership changes.
- Files: `server/room.go:211`, `server/room.go:236`, `server/handler.go:250`, `server/handler.go:295`.
- Risk: shutdown, disconnect, moderation, and room switching can all open slots. Off-by-one or duplicate-admission bugs will show up under concurrency first.

## History Retention Is Daily Only

- Invariant: in-memory room history resets at midnight and recovery only replays today’s log.
- Files: `server/server.go:289`, `server/history.go:16`, `server/history.go:53`, `logger/logger.go:93`.
- Risk: if product expectations evolve toward long-lived room history, current behavior will look like data loss even though it matches the current design.

## Log Format Compatibility

- Invariant: `FormatLogLine()` and `ParseLogLine()` must remain compatible, and old logs without `@room` tags must still parse.
- Files: `models/message.go:107`, `models/message.go:139`.
- Risk: changing log syntax without a compatibility path will break startup recovery and invalidate history tests.

## Heartbeat Side Effects

- Invariant: null-byte probes must remain invisible during healthy operation and must not interfere with prompt redraws.
- Files: `server/server.go:216`, `client/client.go:357`.
- Risk: transport changes that stop ignoring `0x00` or alter deadline behavior can create spurious disconnects or visible garbage in terminals.

## Logger Optionality

- Invariant: the server must still run when logging cannot initialize.
- Files: `main.go:27`, `server/history.go:48`, `logger/logger.go:33`.
- Risk: code that assumes `srv.Logger != nil` will panic or disable recovery paths unexpectedly.

## Operator Is Not a Client

- Invariant: operator commands run with authority but without room membership or a network connection.
- Files: `server/operator.go:31`, `server/operator.go:168`, `server/operator.go:366`.
- Risk: reusing user-command logic blindly can produce invalid assumptions about prompts, rooms, or `client.Client` fields.

## High-Blast-Radius Files

- `server/handler.go`: touches onboarding, queueing, room assignment, disconnect cleanup, admin restore, heartbeat startup.
- `server/commands.go`: touches almost every chat feature and moderation path.
- `client/client.go`: owns output serialization and interactive input behavior.
- `models/message.go`: centralizes event formatting and recovery compatibility.
- `server/room.go`: governs room-local history and admission semantics.

## Audit Questions To Rehearse

- What state is authoritative for active users: `allClients`, `clients`, or `rooms[*].clients`?
- Which code paths write directly to the socket, and why are they exceptions?
- Why does banning one user disconnect others on the same host?
- What user-visible events are persisted and replayed, and which are intentionally dropped?
- How does the server avoid losing partially typed client input during asynchronous broadcasts?
