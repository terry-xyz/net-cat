# Net-Cat Concepts

## Single Source of Truth for Events

- Concept: `models.Message` is the canonical event object for chat messages, joins, leaves, renames, announcements, moderation, and server events.
- Why it matters here: the same object feeds user display (`models/message.go:85`), log serialization (`models/message.go:107`), and startup recovery (`server/history.go:53`).
- Files: `models/message.go`, `logger/logger.go`, `server/history.go`, `server/commands.go`, `server/handler.go`, `server/operator.go`.
- Audit consequence: if you add a new event type and do not update all three paths, the behavior will diverge between live clients, persisted logs, and recovered history.

## Global Client Registry vs Room Registry

- Concept: the server tracks users twice, once globally by username and once inside each room.
- Why it matters here: global lookup powers cross-room features like `/whisper`, global bans, and admin persistence, while room-local maps constrain room broadcasts and room capacity.
- Files: `server/server.go:22`, `server/clients.go:30`, `server/room.go:56`, `server/commands.go:312`, `server/commands.go:398`.
- Audit consequence: any rename, remove, join, or switch operation must keep both registries coherent.

## All Connection Phases Are Tracked

- Concept: `allClients` includes clients in name prompt, room queue, and active chat, not just registered users.
- Why it matters here: shutdown needs to notify everyone, not just users already visible in `s.clients`.
- Files: `server/server.go:25`, `server/clients.go:12`, `server/server.go:100`, `server/handler.go:47`.
- Audit consequence: if a change only consults `s.clients`, queued or onboarding users may leak or miss shutdown notifications.

## Single Writer Pattern for Client Output

- Concept: almost all writes to a client flow through `Client.writeLoop()`.
- Why it matters here: this prevents interleaved writes and enables input continuity redraws.
- Files: `client/client.go:27`, `client/client.go:83`, `client/client.go:236`, `client/client.go:282`, `client/client.go:326`.
- Exceptions: `server/handler.go:51` and `server/handler.go:58` write directly to the raw connection before normal async flow starts; `server/server.go:251` writes a null byte probe during heartbeat.
- Audit consequence: direct writes are deliberate boundary cases. New direct writes can break ordering or deadlock tests built on synchronous pipes.

## Input Continuity

- Concept: when the server sends asynchronous notifications while a client is typing, the client wrapper clears the line, prints the incoming message, then redraws the prompt and partial input.
- Why it matters here: terminal UX stays usable under concurrent events.
- Files: `client/client.go:282`, `client/client.go:326`, `server/handler.go:146`.
- Audit consequence: any prompt format change or writeLoop change can silently break redraw correctness.

## Room Queue Is Per Room, Not Global

- Concept: capacity and waiting lists are attached to each `Room`.
- Why it matters here: the server now supports multiple rooms with independent limits and independent waiting queues.
- Files: `server/room.go:11`, `server/room.go:197`, `server/room.go:211`, `server/handler.go:250`.
- Audit consequence: moderation, shutdown, and queue-admission logic have to consider room-local queues plus cross-room active clients.

## Operator vs Admin

- Concept: promoted admins are still chat clients; the operator is the server terminal itself.
- Why it matters here: operator commands reuse the command registry for names and descriptions, but apply different applicability rules and never have a backing `client.Client`.
- Files: `cmd/commands.go:23`, `server/operator.go:31`, `server/commands.go:556`, `server/operator.go:366`.
- Audit consequence: features that assume every actor has a room or network connection will break for the operator path.

## Recovery Is Rebuild, Not Snapshot Restore

- Concept: the server does not persist in-memory room state directly; it replays today's append-only log file into room histories on startup.
- Why it matters here: only user-visible events are replayed, and only for the current day.
- Files: `logger/logger.go:33`, `models/message.go:107`, `server/history.go:53`, `server/server.go:90`.
- Audit consequence: server-only events, previous-day history, and any event not represented by a parseable log line are intentionally not recovered.

## IP-Based Moderation Is Host-Based

- Concept: moderation extracts the host portion from `host:port` and uses that as the durable key.
- Why it matters here: reconnects from a different source port still match the same ban or kick cooldown.
- Files: `server/moderation.go:11`, `server/commands.go:398`, `server/operator.go:211`.
- Audit consequence: NAT users share a moderation fate; banning one host disconnects other clients from the same host.

## One Big Lock

- Concept: `Server.mu` is the main synchronization primitive for server state.
- Why it matters here: rooms, client registries, admin lists, and moderation maps all sit under one lock.
- Files: `server/server.go:26`, `server/clients.go`, `server/room.go`, `server/admins.go`, `server/moderation.go`.
- Audit consequence: simple to reason about, but high-blast-radius for contention and deadlock if code starts holding the lock across network writes or long-running work.
