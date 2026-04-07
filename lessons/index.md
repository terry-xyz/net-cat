# Net-Cat Study Index

## Purpose

This `lessons/` set is built for audit-speed understanding of the repository. The primary artifacts are the sharded `line-by-line-*.md` files. `concepts.md` names the abstractions you need to reason correctly. `risks.md` names the failure surfaces and high-blast-radius edits.

## System Snapshot

- Entrypoint: `main.go` validates the port, creates the server and logger, starts signal handling, starts the operator terminal, then blocks in `srv.Start()`.
- Core runtime: `server.Server` owns the listener, global client registry, room registry, queue state, moderation state, admin persistence, shutdown coordination, heartbeat scheduling, and midnight history rotation.
- Transport boundary: `client.Client` wraps each `net.Conn`, owns the write goroutine, and preserves partial input when asynchronous messages arrive.
- Event model: `models.Message` is the single representation for displayable events, log lines, and replayable history.
- Persistence: `logger.Logger` writes append-only daily logs, `server.RecoverHistory()` rebuilds in-memory room history from the current day, and `server/admins.go` persists promoted admins in `admins.json`.

## Recommended Reading Order

1. `line-by-line-main-go.md`
2. `line-by-line-server-server-go.md`
3. `line-by-line-server-handler-go.md`
4. `line-by-line-client-client-go.md`
5. `line-by-line-server-room-go.md`
6. `line-by-line-server-clients-go.md`
7. `line-by-line-server-commands-go.md`
8. `line-by-line-server-operator-go.md`
9. `line-by-line-models-message-go.md`
10. `line-by-line-logger-logger-go.md`
11. `line-by-line-server-history-go.md`
12. `line-by-line-server-admins-go.md`
13. `line-by-line-server-moderation-go.md`
14. `concepts.md`
15. `risks.md`

## Hotspots

- `server/handler.go`: onboarding, room selection, queue entry, message loop, disconnect cleanup.
- `server/commands.go`: almost all user-visible behavior after onboarding.
- `server/server.go`: shutdown, heartbeat, acceptance loop, queue totals.
- `client/client.go`: transport safety, backpressure handling, input continuity.
- `server/room.go`: room-local state, history, queue admission.

## Shards

- `line-by-line-main-go.md`: process bootstrap and port validation.
- `line-by-line-client-client-go.md`: connection wrapper, write loop, interactive input.
- `line-by-line-cmd-commands-go.md`: command registry and parsing.
- `line-by-line-logger-logger-go.md`: log file lifecycle and date rollover.
- `line-by-line-models-message-go.md`: event formatting, display, and log parsing.
- `line-by-line-server-server-go.md`: server state object, startup, shutdown, heartbeat, midnight watcher.
- `line-by-line-server-handler-go.md`: onboarding, room entry, queue waiting, and validation.
- `line-by-line-server-commands-go.md`: user commands and cross-room behavior.
- `line-by-line-server-room-go.md`: room state, history, and queue implementation.
- `line-by-line-server-clients-go.md`: global client registry and broadcast helpers.
- `line-by-line-server-admins-go.md`: admin persistence to disk.
- `line-by-line-server-history-go.md`: history reset and recovery from logs.
- `line-by-line-server-moderation-go.md`: IP extraction and ban/kick cooldown storage.
- `line-by-line-server-operator-go.md`: operator terminal command surface.

## How To Use This Pack

- Start with the runtime shards.
- Use `concepts.md` when a behavior depends on an abstraction spanning multiple files.
- Use `risks.md` before making changes.
