# line-by-line: server/server.go

## File Purpose

Defines the `Server` state object and the top-level lifecycle operations: startup, accept loop, shutdown, queue counting, heartbeat, and midnight watcher.

## Why This File Matters

This file is the runtime skeleton. It decides what state exists and when background goroutines start and stop.

## Dependencies In and Out

- Inbound: `main.go`.
- Outbound: `clients.go`, `handler.go`, `history.go`, `room.go`, `moderation.go`, `admins.go`.

## Ordered Walkthrough

- `server/server.go:16-21`: `QueueEntry` is the per-waiter synchronization object; closing `admit` is the admission signal.
- `server/server.go:22-53`: `Server` collects all server-wide state. The important split is between `clients` (registered usernames), `allClients` (all lifecycle phases), and `rooms` (room-local state).
- `server/server.go:54-77`: `New` initializes maps, default configuration, operator output, and the default room. It ensures `general` exists from the start.
- `server/server.go:78-99`: `Start` opens the listener, prints the listening message, logs a server-start event, loads persisted admins, recovers today’s history, starts midnight rotation, then enters the accept loop and waits for shutdown completion.
- `server/server.go:100-153`: `Shutdown` is guarded by `sync.Once`. It closes `quit`, stops the listener, broadcasts goodbye to every tracked connection, waits up to `ShutdownTimeout`, force-closes stragglers, logs server shutdown, closes the logger, and closes `shutdownDone`.
- `server/server.go:154-171`: `acceptLoop` accepts new TCP connections until listener shutdown or transient accept errors.
- `server/server.go:172-191`: room-capacity constant plus queue-length and queue-removal helpers. Queue removal delegates to room-level logic.
- `server/server.go:192-215`: `IsShuttingDown` exposes `quit` state to callers that need to stop mid-flow.
- `server/server.go:216-288`: `startHeartbeat` is the dead-client detector. It skips recently active clients, probes by writing a null byte from a separate goroutine, disconnects on write failure or timeout, warns on slow success, and exits cleanly when either the client or server is done.
- `server/server.go:289-304`: `startMidnightWatcher` waits until the next local midnight, clears history, and repeats until shutdown.

## Key Takeaways

- `Server` is a single-lock, multi-goroutine state machine.
- Shutdown and heartbeat are first-class lifecycle features, not add-ons.
- Midnight logic resets in-memory history but leaves file rollover to the logger.

## Audit Notes

- If you change lifecycle ordering in `Start`, inspect admin restore, recovery, and logger assumptions.
- If you change `Shutdown`, inspect queued-client behavior, handler cleanup, and server tests around idempotence and timing.
