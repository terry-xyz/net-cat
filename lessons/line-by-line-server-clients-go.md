# line-by-line: server/clients.go

## File Purpose

Global connection and username registry helpers.

## Ordered Walkthrough

- `server/clients.go:12-29`: `TrackClient` and `UntrackClient` manage `allClients`, which exists for lifecycle-wide coverage beyond registered users.
- `server/clients.go:30-46`: `RegisterClient` performs the atomic uniqueness check, sets `Username`, `JoinTime`, and initial `lastActivity`, then inserts the client into the global username map.
- `server/clients.go:47-60`: `RemoveClient` removes the user from both global and room-local registries.
- `server/clients.go:61-88`: simple getters plus reserved-name check.
- `server/clients.go:89-112`: `RenameClient` updates both global username map and current room map atomically.
- `server/clients.go:113-127`: `GetClientsByIP` is used by ban logic to find collateral users sharing a host.
- `server/clients.go:128-145`: global broadcast helpers iterate the registered-user map, not `allClients`.

## Audit Notes

- `BroadcastAll` intentionally excludes queued or onboarding users; shutdown is the path that covers those via `allClients`.
