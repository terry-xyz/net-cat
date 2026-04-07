# line-by-line: server/moderation.go

## File Purpose

Minimal IP-based moderation helpers.

## Ordered Walkthrough

- `server/moderation.go:11-19`: `extractHost` normalizes `host:port` strings down to the host key used by moderation maps.
- `server/moderation.go:20-27`: `AddKickCooldown` records a five-minute temporary block.
- `server/moderation.go:28-36`: `AddBanIP` records a session-long host ban.
- `server/moderation.go:37-51`: `IsIPBlocked` checks bans first, then kick cooldowns, cleaning up expired cooldowns on read.

## Audit Notes

- This file is intentionally tiny, but its host-normalization choice drives the collateral behavior of bans and reconnect blocking.
