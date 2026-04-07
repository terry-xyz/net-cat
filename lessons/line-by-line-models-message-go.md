# line-by-line: models/message.go

## File Purpose

Defines the event model and the three projections of an event: what clients see, what logs persist, and what recovery parses.

## Why This File Matters

This is the semantic center of the system. Any mismatch here propagates everywhere.

## Dependencies In and Out

- Inbound: virtually the whole server and logger stack.
- Outbound: only formatting/parsing helpers from the standard library.

## Ordered Walkthrough

- `models/message.go:10-20`: `MessageType` enumerates the supported event categories.
- `models/message.go:22-39`: `Message` stores shared event fields. The comment block is important because field meanings vary by `Type`.
- `models/message.go:42-82`: formatter helpers produce user-facing strings for timestamps, chat prompts, joins/leaves, rename notices, announcements, moderation notices, and whispers.
- `models/message.go:85-104`: `Display` maps a `Message` to what clients should see. `MsgServerEvent` intentionally returns raw content because server-only events are not decorated like chat.
- `models/message.go:107-136`: `FormatLogLine` maps the same event into a parseable log syntax. Room tags appear for every event except `MsgServerEvent`, leave events default to `voluntary`, and each event type gets a distinct keyword.
- `models/message.go:139-242`: `ParseLogLine` is the inverse parser. It trims whitespace, parses the timestamp manually, handles optional `@room` tags, decodes each keyword form, preserves backward compatibility for old lines with no room tag, and rejects malformed input with explicit errors.

## Key Takeaways

- One message model supports both live UX and replayable persistence.
- `FormatLogLine` and `ParseLogLine` are a pair and should be changed together.

## Audit Notes

- New event types require changes to `MessageType`, `Display`, `FormatLogLine`, `ParseLogLine`, and the recovery tests.
