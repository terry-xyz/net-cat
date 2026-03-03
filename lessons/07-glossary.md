# Lesson 07: Glossary

---

## Start Here: Essential Terms

These terms appear constantly. Master these first.

| Term | Plain English | Example in This Codebase |
|------|---------------|--------------------------|
| Goroutine | A lightweight background thread | `go s.handleConnection(conn)` |
| Channel | A pipe for goroutines to communicate | `msgChan chan writeMsg` |
| Mutex | A lock that prevents simultaneous access | `s.mu.Lock()` / `s.mu.Unlock()` |
| Struct | A container grouping related data | `type Server struct { ... }` |
| Interface | A contract: "any type with these methods" | `io.Writer`, `io.Reader` |
| Defer | "Run this when the function exits" | `defer s.UntrackClient(c)` |
| Slice | A dynamically-sized list | `[]models.Message` |
| Map | A dictionary (key → value) | `map[string]*client.Client` |
| Pointer | A reference to data, not a copy | `*client.Client` |
| Package | A folder of related Go files | `package server` |

---

## Domain Terms (The Problem)

These terms describe the chat system's concepts:

| Term | Meaning |
|------|---------|
| Client | A user connected via TCP. Wraps a `net.Conn` with state. |
| Room | A chat channel. Users in the same room see each other's messages. |
| Default Room | The "general" room — always exists, can't be deleted. |
| Queue | A waiting line for rooms at max capacity (10 users). |
| Operator | The person running the server binary. Has full authority via stdin. |
| Admin | A user promoted by the operator. Can kick, ban, mute. |
| Moderation | Actions like kick, ban, mute, promote, demote. |
| Kick | Remove a user and block their IP for 5 minutes. |
| Ban | Remove a user and block their IP for the entire server session. |
| Mute | Prevent a user from sending chat messages (they can still read). |
| Whisper | A private message between two users (not logged, not stored). |
| Announcement | A server-wide message from an admin/operator. |
| Heartbeat | A periodic null-byte probe to detect dead connections. |
| Echo Mode | After onboarding, the server echoes typed characters back. |
| Input Continuity | Preserving partial typed text when a message arrives. |
| Onboarding | The name validation + room selection flow for new connections. |

---

## Code Terms (The Implementation)

| Term | Meaning | Location |
|------|---------|----------|
| `writeLoop` | Single goroutine that writes to a TCP connection | `client/client.go:236` |
| `msgChan` | Buffered channel (4096) for queueing outgoing messages | `client/client.go:46` |
| `done` | Signal channel — closed when a client disconnects | `client/client.go:47` |
| `quit` | Signal channel — closed when the server shuts down | `server/server.go:31` |
| `admit` | Signal channel — closed when a queued user is admitted | `server/server.go:18` |
| `allClients` | Every TCP connection in any phase (onboarding, queued, active) | `server/server.go:26` |
| `clients` | Only fully registered users (name assigned, in a room) | `server/server.go:25` |
| `recordRoomEvent` | Adds to room history + writes to log file | `server/room.go:131` |
| `BroadcastRoom` | Sends a message to all users in a specific room | `server/room.go:71` |
| `BroadcastAllRooms` | Sends a message to all users across all rooms | `server/room.go:99` |
| `FormatLogLine` | Converts a Message to a parseable log string | `models/message.go:107` |
| `ParseLogLine` | Converts a log string back to a Message | `models/message.go:139` |
| `RecoverHistory` | On startup, replays today's log into memory | `server/history.go:53` |
| `ensureFile` | Opens/switches the log file for the current date | `logger/logger.go:93` |
| `extractHost` | Gets the IP from a "host:port" string | `server/moderation.go:11` |

---

## Variable Naming (Why Names Look Weird)

| Variable | What It Means | Why This Name |
|----------|---------------|---------------|
| `c` | A `*client.Client` | Short for "client" — used everywhere |
| `s` | A `*Server` (method receiver) | Standard Go convention for receivers |
| `r` | A `*Room` | Short for "room" |
| `l` | A `*Logger` | Short for "logger" |
| `mu` | A `sync.Mutex` or `sync.RWMutex` | Standard Go abbreviation for "mutual exclusion" |
| `rn` | Room name (string) | Short for "room name" |
| `cc` | A collateral client (banned IP) | "collateral client" — secondary target in bans |
| `b` | A single byte | Standard for byte-level processing |
| `ts` | Timestamp | Common abbreviation |
| `idx` | Index into a string/slice | Common abbreviation |
| `def` | A `CommandDef` | Short for "definition" |

---

## Abbreviations Decoded

| Abbreviation | Full Form |
|-------------|-----------|
| `TOCTOU` | Time-of-Check-Time-of-Use (a race condition) |
| `FIFO` | First In, First Out (queue ordering) |
| `ANSI` | American National Standards Institute (terminal escape codes) |
| `CRLF` | Carriage Return + Line Feed (`\r\n`) |
| `LF` | Line Feed (`\n` only) |
| `TCP` | Transmission Control Protocol |
| `IP` | Internet Protocol (address) |
| `NAT` | Network Address Translation (multiple users behind one IP) |
| `RW` | Read-Write (as in `sync.RWMutex`) |
| `EOF` | End of File (or end of stream) |
| `ASCII` | American Standard Code for Information Interchange |

---

## File Types & Suffixes

| File | Purpose |
|------|---------|
| `*.go` | Go source code |
| `*_test.go` | Go test files (run with `go test`) |
| `go.mod` | Module definition (name, Go version, dependencies) |
| `*.log` | Daily chat log files |
| `admins.json` | Persisted admin username list |
| `.gitignore` | Files git should not track |

---

## Magic Numbers (Why These Values?)

| Value | Where | Why |
|-------|-------|-----|
| `8989` | `main.go:12` | Default port — memorable, high enough to avoid privilege requirements |
| `10` | `server/server.go:172` | Max users per room — balances usability with readability |
| `4096` | `client/client.go:13` | Write channel buffer — large enough for burst traffic without blocking |
| `2048` | `server/commands.go:19` | Max message length — prevents abuse while allowing long messages |
| `32` | `server/handler.go:358` | Max name length — prevents visual abuse in the chat display |
| `5 * time.Minute` | `server/moderation.go:24` | Kick cooldown — long enough to be meaningful, short enough to not be permanent |
| `5 * time.Second` | `server/server.go:119,223` | Shutdown timeout and heartbeat timeout |
| `10 * time.Second` | `server/server.go:219` | Heartbeat interval — frequent enough to detect ghosts, rare enough to not spam |
| `0x21-0x7E` | `server/handler.go:360` | Printable ASCII range (! through ~) — no control chars or non-ASCII |
| `0x00` | `server/server.go:244` | Null byte — invisible to terminal emulators, used as heartbeat probe |
| `0600` | `logger/logger.go:101` | File permissions: owner read+write only (no group/other access) |
| `200ms` | `server/server.go:140` | Post-shutdown wait — allows deferred cleanup goroutines to finish |

---

## Function Naming Patterns

| Pattern | Meaning | Examples |
|---------|---------|---------|
| `New*` | Constructor — creates and returns a new instance | `NewClient`, `New` (server, logger) |
| `Get*` | Getter — returns a value, usually read-locked | `GetClient`, `GetRoomHistory`, `GetLastInput` |
| `Set*` | Setter — writes a value, usually mutex-protected | `SetAdmin`, `SetMuted`, `SetEchoMode` |
| `Is*` | Boolean check — returns true/false | `IsAdmin`, `IsMuted`, `IsClosed`, `IsIPBlocked` |
| `cmd*` | Client-facing command handler | `cmdKick`, `cmdBan`, `cmdWhisper` |
| `operatorCmd*` | Operator terminal command handler | `operatorCmdKick`, `operatorCmdBan` |
| `Format*` | Creates a display string from data | `FormatChat`, `FormatJoin`, `FormatLogLine` |
| `Parse*` | Reconstructs data from a string | `ParseCommand`, `ParseLogLine` |
| `Broadcast*` | Sends to multiple clients | `BroadcastRoom`, `BroadcastAllRooms` |
| `Add*` | Adds something to a collection | `AddAdmin`, `AddKickCooldown`, `AddRoomHistory` |
| `Remove*` | Removes something from a collection | `RemoveClient`, `RemoveAdmin` |
| `start*` (lowercase) | Launches a long-running goroutine | `startHeartbeat`, `startMidnightWatcher` |

---

## Error Messages Decoded

| Error Message | What Went Wrong | Where It Comes From |
|---------------|----------------|---------------------|
| "Name is already taken." | Another user has this name | `server/handler.go:95` |
| "Name cannot be empty." | User pressed Enter without typing | `server/handler.go:339` |
| "Name must contain only printable characters." | Non-ASCII or control characters | `server/handler.go:362` |
| "Insufficient privileges." | User tried an admin command | `server/commands.go:55` |
| "You are muted." | Muted user tried to chat | `server/commands.go:25` |
| "Message too long (max 2048 characters)." | Chat message exceeds limit | `server/commands.go:20` |
| "You are banned from this server." | IP is on the ban list | `server/moderation.go:42` |
| "You are temporarily blocked." | IP has active kick cooldown | `server/moderation.go:46` |
| "User 'X' not found." | Target of command doesn't exist | Various command handlers |
| "Room 'X' is full." | Room at 10 users | `server/commands.go:169` |
| "Connection unstable..." | Heartbeat probe was slow | `server/server.go:259` |
| "Server is shutting down." | Ctrl+C or signal received | `server/server.go:112` |

---

## Glossary Complete!

**You now have:**
- Essential vocabulary for Go and this codebase
- Decoder rings for every abbreviation and magic number
- Quick reference for error messages and their sources
- Understanding of naming conventions used throughout

**Congratulations!** You're ready to:
1. Navigate any part of this codebase with confidence
2. Understand WHY the code is written the way it is
3. Debug issues by tracing data flow
4. Communicate using shared vocabulary
5. Extend the codebase without introducing the gotchas from Lesson 06

**The journey from "vibecoding" to understanding is complete.**
