# line-by-line: server/commands.go

## File Purpose

Implements all user-facing slash commands plus normal chat-message handling.

## Why This File Matters

Most feature work lands here. It is also where room-local behavior and global behavior meet.

## Dependencies In and Out

- Inbound: `server/handler.go`.
- Outbound: client state, room state, history/logging, admin persistence, moderation IP helpers.

## Ordered Walkthrough

- `server/commands.go:14-45`: `handleChatMessage` enforces non-empty trimmed content, 2048-character max length, mute status, room-event recording, room broadcast, activity timestamp update, and prompt redraw.
- `server/commands.go:46-95`: `dispatchCommand` looks up the command registry, enforces privilege, routes by command name, and returns `true` only for `/quit`.
- `server/commands.go:96-134`: `/list` is room-scoped. It snapshots current room members, computes idle time from `lastActivity`, insertion-sorts names, and sends private output only to the requester.
- `server/commands.go:135-150`: `/rooms` lists all rooms, counts, and marks the caller’s current room.
- `server/commands.go:151-179`: `/switch` validates target room, rejects no-op switches and full rooms, then delegates to `switchClientRoom`.
- `server/commands.go:180-208`: `/create` validates room names and rejects creating an already-existing room.
- `server/commands.go:209-249`: `switchClientRoom` records a leave event in the old room, moves the client under lock, opens old-room queue slots, deletes the old room if empty, replays new-room history, records a join in the new room, and sends a confirmation.
- `server/commands.go:250-263`: `/help` filters the shared command registry by the caller’s privilege.
- `server/commands.go:264-311`: `/name` validates syntax, reserved names, self-rename, and conflicts, then renames globally and room-locally, renames persisted admin entries when needed, records a room event, broadcasts the change, and redraws the prompt.
- `server/commands.go:312-356`: `/whisper` is cross-room. It parses recipient plus message, rejects empty/oversized/self whispers, looks up the target in the global registry, then sends sender and recipient private messages without touching history.
- `server/commands.go:357-397`: `/kick` is same-room only. It force-sets disconnect reason, removes the target from registries, records a moderation event, broadcasts moderation output in the room, notifies the target, closes the target, starts kick cooldown on the IP, and admits one queued user from that room.
- `server/commands.go:398-467`: `/ban` is global. It removes the target, records and broadcasts moderation, bans the IP host, disconnects collateral clients on the same host across rooms, removes queued users from the same host, and admits opened queue slots in every affected room.
- `server/commands.go:468-533`: `/mute` and `/unmute` are global state toggles on the target client. They log room events using the target’s current room and broadcast moderation notices to all rooms.
- `server/commands.go:534-555`: `/announce` logs an announcement into every room’s history and broadcasts the rendered announcement globally.
- `server/commands.go:556-622`: `/promote` and `/demote` mutate the target’s admin bit, persist the admin list, notify the target, record a moderation event, and broadcast the result globally.

## Key Takeaways

- Commands either act room-locally (`/list`, `/switch`, `/kick`) or globally (`/whisper`, `/ban`, `/announce`, `/promote`).
- Prompt redraw after each command is a consistent UX invariant.
- Moderation commands mix client-state mutation, room/global broadcast, and persistence side effects.

## Audit Notes

- `/ban` has the largest blast radius. Any change there requires re-reading NAT collateral logic, queue removal, and room reopening behavior.
- Rename and room-switch logic are the main registry-coherence hazards.
