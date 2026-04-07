# line-by-line: server/room.go

## File Purpose

Defines room-local state and the helper methods that operate on room membership, history, broadcasts, and room queues.

## Why This File Matters

This file implements the server’s room abstraction. Multi-room correctness depends on it.

## Dependencies In and Out

- Inbound: `server/handler.go`, `server/commands.go`, `server/history.go`, `server/server.go`.
- Outbound: `models.Message`, `client.Client`.

## Ordered Walkthrough

- `server/room.go:11-17`: `Room` holds room name, room-local client map, room-local history, and room-local waiting queue.
- `server/room.go:18-27`: `newRoom` initializes the client map for a room.
- `server/room.go:28-38`: `getOrCreateRoom` is the internal constructor/cache lookup and requires `s.mu` write lock.
- `server/room.go:39-55`: `deleteRoomIfEmpty` removes empty non-default rooms only when both client list and queue are empty.
- `server/room.go:56-70`: `JoinRoom` removes the client from the previous room map if needed, inserts into the target room, and updates `c.Room`.
- `server/room.go:71-109`: room-scoped and all-room broadcast helpers read-lock the room or global client maps and enqueue messages to recipients.
- `server/room.go:110-139`: room-history helpers either copy history out, append to history, or route a message through history and logger together with `recordRoomEvent`.
- `server/room.go:140-196`: room-query helpers produce sorted room names, counts, and sorted member names.
- `server/room.go:197-210`: `checkRoomCapacity` treats nonexistent rooms as available and existing rooms as capped by `MaxActiveClients`.
- `server/room.go:211-255`: queue helpers admit the first live waiting client, send queue-position updates to remaining waiters, and remove a specific queue entry with re-numbering.
- `server/room.go:256-279`: `RemoveFromAllRoomQueuesByIP` removes queued users from every room whose host IP matches, then re-sends queue positions in affected rooms.

## Key Takeaways

- Rooms are lightweight containers under the server’s single mutex.
- History and queueing are room-local, but username uniqueness remains global.

## Audit Notes

- If you add room metadata or per-room policies, this file is the natural home, but keep in mind every room operation currently assumes one global lock.
