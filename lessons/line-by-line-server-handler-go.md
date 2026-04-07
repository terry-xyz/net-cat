# line-by-line: server/handler.go

## File Purpose

Owns the TCP connection lifecycle from first byte to disconnect cleanup.

## Why This File Matters

This is the highest-blast-radius file in the repo. It ties together onboarding, validation, queueing, room join, history replay, prompt setup, heartbeat startup, command dispatch, chat dispatch, and disconnect cleanup.

## Dependencies In and Out

- Inbound: called from `server/server.go:167`.
- Outbound: `client`, `cmd`, `models`, room helpers, client registry helpers, moderation helpers.

## Ordered Walkthrough

- `server/handler.go:15-39`: banner and prompt constants define the user’s first interaction and room-selection UX.
- `server/handler.go:40-61`: `handleConnection` starts by enabling TCP keepalive when possible, wrapping the raw connection in `client.NewClient`, tracking it in `allClients`, checking IP bans/cooldowns before the banner, and rejecting new connections during shutdown.
- `server/handler.go:63-96`: onboarding loop. The client receives banner plus `NamePrompt`, `ReadLine()` collects name input, `validateName` enforces syntax, reserved-name and uniqueness checks run, and the loop retries until a good name is registered or the connection dies.
- `server/handler.go:98-103`: returning admins are auto-promoted based on the persisted admin list.
- `server/handler.go:105-116`: room selection occurs before active-room membership. Empty input joins the default room; invalid names retry.
- `server/handler.go:118-124`: capacity and queue handling happen before joining the room. Failure here removes the username registration and closes the connection.
- `server/handler.go:126-174`: after successful join, a deferred cleanup closure becomes responsible for normal disconnects. It distinguishes voluntary/drop disconnects from kick/ban, logs leave events, broadcasts room leave notices, admits the next queued client, deletes empty rooms, and closes the client.
- `server/handler.go:176-183`: room history is replayed before echo mode starts so new joiners get prior context cleanly.
- `server/handler.go:185-198`: echo mode is enabled, first prompt is sent via `SendPrompt`, join event is recorded, and room peers are notified.
- `server/handler.go:200-212`: heartbeat state is initialized, heartbeat goroutine starts, and the main interactive loop begins. Each incoming line refreshes heartbeat input time, goes through command parsing if slash-prefixed, otherwise becomes a chat message.
- `server/handler.go:214-249`: room-selection helpers list rooms and validate room names. Blank room input resolves to `DefaultRoom`.
- `server/handler.go:250-336`: queue path. Full rooms append a `QueueEntry`, offer `yes/no`, wait for admission, disconnection, or shutdown, then reset the scanner after using a read deadline to cancel the monitoring goroutine.
- `server/handler.go:337-381`: name and room validators enforce non-empty, no-space, printable-ASCII, and max-32-length rules. Room names are slightly looser because they do not reject spaces explicitly, only empty/non-printable/too-long input.

## Key Takeaways

- Connection handling is a staged pipeline: pre-checks, name registration, admin restore, room choice, optional queue, room join, history replay, interactive loop, deferred cleanup.
- `allClients` tracking begins before user registration and ends after cleanup.
- The deferred cleanup closure is the main reason disconnect semantics stay consistent across many exit paths.

## Audit Notes

- If you touch the defer block, inspect kick/ban behavior carefully; those paths intentionally pre-handle some cleanup.
- If you touch queue waiting, inspect scanner reset and deadline cancellation, because that path is subtle and test-heavy.
