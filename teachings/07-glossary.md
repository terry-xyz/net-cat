# Glossary

This glossary is organized by **frequency** — the terms you'll encounter most often are first. Master the "Essential Terms" section before moving on.

---

## Start Here: Essential Terms

These terms appear constantly throughout the codebase. Master these first.

| Term | Plain English | Example in This Codebase |
|------|---------------|--------------------------|
| **goroutine** | A lightweight thread — code running in the background | `go s.handleConnection(conn)` |
| **channel** | A pipe for safely passing data between goroutines | `msgChan chan writeMsg` |
| **mutex** | A lock preventing concurrent access to shared data | `s.mu.Lock()` / `s.mu.Unlock()` |
| **struct** | A bundle of related data (like a form with fields) | `type Client struct { ... }` |
| **interface** | A contract: "anything that can do X" | `net.Conn` (anything with Read/Write) |
| **defer** | "Run this when the function exits" — guaranteed cleanup | `defer s.mu.Unlock()` |
| **error** | Go's explicit "something went wrong" return value | `if err != nil { return err }` |
| **nil** | "Nothing" / "empty" / "doesn't exist" | `if l == nil { return }` |
| **pointer** (`*`) | A reference to the original, not a copy | `*Server`, `*Client` |
| **method** | A function attached to a type | `func (s *Server) Start()` |

---

## Domain Terms (The Problem Space)

These terms describe the **chat system** — the problem this code solves.

| Term | Plain English | Where Used |
|------|---------------|------------|
| **client** | A person connected to the chat | `client/client.go` |
| **server** | The program that manages all connections | `server/server.go` |
| **operator** | The person running the server (types commands in the terminal) | `server/server.go:739` |
| **admin** | A promoted user with moderation powers | `server/server.go:52` |
| **broadcast** | Send a message to all connected clients | `server/server.go:311` |
| **whisper** | A private message between two users | `server/handler.go:506` |
| **kick** | Remove a user with a temporary IP cooldown (5 min) | `server/handler.go:551` |
| **ban** | Remove a user with a permanent IP block (for the session) | `server/handler.go:586` |
| **mute** | Prevent a user from sending chat messages | `server/handler.go:654` |
| **queue** | A waiting line when the server is full (max 10 active) | `server/server.go:384` |
| **onboarding** | The process of a new client: connect → name → join | `server/handler.go:37` |
| **heartbeat** | A periodic health check to detect dead connections | `server/server.go:644` |
| **history** | In-memory list of past messages (sent to new clients) | `server/server.go:280` |
| **prompt** | The `[timestamp][username]:` prefix shown before typing | `models/message.go:51` |
| **banner** | The ASCII penguin art shown on connect | `server/handler.go:15` |
| **moderation** | Admin actions: kick, ban, mute, unmute, promote, demote | `server/handler.go:549+` |

---

## Code Terms (The Implementation)

These terms describe **how** the code works — the patterns and structures used.

| Term | Plain English | Where Used |
|------|---------------|------------|
| **writeLoop** | The goroutine that writes to a client's TCP connection | `client/client.go:235` |
| **echoMode** | Character-at-a-time input handling (vs. line-buffered) | `client/client.go:56` |
| **inputBuf** | Server-side tracking of what the user has typed so far | `client/client.go:59` |
| **input continuity** | Preserving partial typing when incoming messages arrive | `client/client.go:284` |
| **enqueue** | Put a message in a client's write channel (non-blocking) | `client/client.go:92` |
| **done channel** | A `chan struct{}` closed to signal "stop" | `client/client.go:46` |
| **quit channel** | Server-wide shutdown signal | `server/server.go:40` |
| **admit channel** | Per-queue-entry signal: "you can enter now" | `server/server.go:28` |
| **acceptLoop** | The main loop that waits for new TCP connections | `server/server.go:159` |
| **recordEvent** | Save to both in-memory history AND the log file | `server/server.go:303` |
| **allClients** | Map of ALL connections (including pre-registration) | `server/server.go:36` |
| **clients** | Map of only registered (named) clients | `server/server.go:35` |
| **RWMutex** | A lock that allows many readers OR one writer | `server/server.go:37` |
| **closeOnce** | `sync.Once` ensuring `Close()` runs only once | `client/client.go:47` |
| **shutdownOnce** | `sync.Once` ensuring `Shutdown()` runs only once | `server/server.go:41` |

---

## Variable Naming (Why Names Look Weird)

Go has strong naming conventions. Here's why variables are named the way they are.

| Name | Why This Name | Convention |
|------|---------------|------------|
| `s` | Receiver for `*Server` methods | Go convention: single-letter receiver |
| `c` | Receiver for `*Client` methods (or a client variable) | Single-letter for receivers; short for locals |
| `mu` | Short for "mutex" | Universal Go convention |
| `err` | An error return value | Universal Go convention |
| `ok` | Boolean result from type assertion or map lookup | Universal Go convention |
| `n` | Number of bytes read/written | Universal in I/O code |
| `b` | A single byte | Common in byte-processing code |
| `ts` | Timestamp | Domain abbreviation |
| `conn` | A network connection (`net.Conn`) | Standard Go networking name |
| `buf` | A byte buffer | Universal in I/O code |
| `idx` | An index position | Common abbreviation |
| `l` | Logger instance | Single-letter for short-lived variables |
| `f` | File handle | Single-letter for short-lived variables |
| `cc` | "Collateral client" — same-IP client during ban | Context-specific; see `/ban` code |
| `wm` | "Write message" prefix for constants | `wmMessage`, `wmPrompt`, `wmEcho`, etc. |

---

## Abbreviations Decoded

| Abbreviation | Full Form | Meaning |
|--------------|-----------|---------|
| `TCP` | Transmission Control Protocol | Reliable network communication |
| `IP` | Internet Protocol | Network address (e.g., `192.168.1.5`) |
| `EOF` | End Of File | The connection was closed cleanly |
| `ANSI` | American National Standards Institute | Terminal control codes (colors, cursor movement) |
| `ASCII` | American Standard Code for Information Interchange | Character encoding (A=65, a=97, 0=48) |
| `NAT` | Network Address Translation | Multiple devices sharing one public IP |
| `TOCTOU` | Time-Of-Check-To-Time-Of-Use | A race condition between checking and acting |
| `I/O` | Input/Output | Reading from / writing to something |
| `OS` | Operating System | The software managing hardware (Windows, Linux, macOS) |
| `RW` | Read-Write | As in `sync.RWMutex` — multiple readers OR one writer |
| `LF` | Line Feed | The `\n` character (ASCII 10) |
| `CR` | Carriage Return | The `\r` character (ASCII 13) |

---

## File Types & Suffixes

| Extension | What It Is | Example |
|-----------|-----------|---------|
| `.go` | Go source code | `main.go`, `server.go` |
| `_test.go` | Go test file (only runs with `go test`) | `handler_test.go` |
| `.mod` | Go module definition (project name + Go version) | `go.mod` |
| `.sum` | Checksum file for dependency verification | `go.sum` |
| `.json` | JSON data file | `admins.json` |
| `.log` | Log file (plain text, one event per line) | `chat_2026-02-24.log` |
| `.md` | Markdown documentation | `README.md` |
| `.tmp` | Temporary file (used for atomic writes) | `.admins.json.tmp` |

---

## Magic Numbers (Why These Values?)

| Value | Where | Why This Number |
|-------|-------|-----------------|
| `8989` | `main.go:12` | Default port — easy to remember, above 1024 (no root needed) |
| `10` | `server/server.go:387` | Max active clients — project specification limit |
| `32` | `server/handler.go:323` | Max username length — keeps display formatting manageable |
| `2048` | `server/handler.go:341` | Max message length — prevents flooding |
| `4096` | `client/client.go:13` | Message channel buffer size — large enough for history replay |
| `4096` | `client/client.go:14` | Max interactive input buffer — matches channel size |
| `1048576` | `client/client.go:12` | Scanner buffer limit (1 MB) — handles very long lines |
| `5 * time.Second` | `server/server.go:41,651` | Heartbeat probe timeout / TCP keepalive period |
| `10 * time.Second` | `server/server.go:647` | Heartbeat check interval |
| `5 * time.Minute` | `server/server.go:491` | Kick cooldown duration |
| `50 * time.Millisecond` | `server/server.go:134` | Shutdown poll interval — fast enough to feel responsive |
| `200 * time.Millisecond` | `server/server.go:145` | Post-force-close sleep — gives deferred cleanup time to run |
| `0x7F` | `client/client.go:358` | ASCII DEL (delete/backspace on many terminals) |
| `0x08` | `client/client.go:358` | ASCII BS (backspace) |
| `0x00` | `client/client.go:364` | Null byte — heartbeat probe; invisible to terminals |
| `0x20` | `client/client.go:367` | ASCII space — lowest printable character |
| `0x7E` | `client/client.go:367` | ASCII tilde `~` — highest printable ASCII character |
| `0600` | `server/server.go:586` | File permission: owner read+write only (no group/other) |
| `0700` | `logger/logger.go:25` | Directory permission: owner all, no group/other |

---

## Function Naming Patterns

| Pattern | Meaning | Examples |
|---------|---------|---------|
| `Get*` | Returns data, doesn't modify anything | `GetClient()`, `GetHistory()`, `GetLastInput()` |
| `Set*` | Modifies a single field | `SetMuted()`, `SetAdmin()`, `SetEchoMode()` |
| `Is*` | Returns a boolean (yes/no question) | `IsAdmin()`, `IsMuted()`, `IsClosed()`, `IsShuttingDown()` |
| `Add*` | Inserts into a collection | `AddHistory()`, `AddAdmin()`, `AddKickCooldown()` |
| `Remove*` | Deletes from a collection | `RemoveClient()`, `RemoveAdmin()`, `RemoveFromQueueByIP()` |
| `Format*` | Converts data to a display string | `FormatChat()`, `FormatJoin()`, `FormatTimestamp()` |
| `Parse*` | Converts a string to structured data | `ParseCommand()`, `ParseLogLine()` |
| `cmd*` | A chat command handler (client-facing) | `cmdKick()`, `cmdList()`, `cmdHelp()` |
| `operator*` | An operator terminal handler | `operatorCmdKick()`, `operatorCmdList()` |
| `start*` | Launches a long-running goroutine | `startHeartbeat()`, `startMidnightWatcher()` |
| `Record*` / `record*` | Save to history + log | `recordEvent()` |
| `Load*` | Read from disk into memory | `LoadAdmins()` |
| `Save*` | Write from memory to disk | `SaveAdmins()` |
| `Recover*` | Rebuild memory state from disk | `RecoverHistory()` |

---

## Message Types Decoded

| Type | Log Keyword | When It Happens | Display Format |
|------|-------------|-----------------|----------------|
| `MsgChat` | `CHAT` | Someone sends a message | `[timestamp][Alice]:hello` |
| `MsgJoin` | `JOIN` | Someone enters the chat | `Alice has joined our chat...` |
| `MsgLeave` | `LEAVE` | Someone exits the chat | `Alice has left our chat...` |
| `MsgNameChange` | `NAMECHANGE` | Someone uses `/name` | `Alice changed their name to Bob` |
| `MsgAnnouncement` | `ANNOUNCE` | Admin uses `/announce` | `[ANNOUNCEMENT]: message` |
| `MsgModeration` | `MOD` | kick/ban/mute/unmute/promote/demote | `Bob was kicked by Alice` |
| `MsgServerEvent` | `SERVER` | Internal server events (start, stop) | Not shown to clients |

---

## Privilege Levels Decoded

| Level | Who Has It | What They Can Do |
|-------|-----------|-----------------|
| `PrivUser` (0) | Every connected client | `/list`, `/quit`, `/name`, `/whisper`, `/help` |
| `PrivAdmin` (1) | Promoted users (via `/promote`) | All user commands + `/kick`, `/ban`, `/mute`, `/unmute`, `/announce` |
| `PrivOperator` (2) | Server terminal only | All commands + `/promote`, `/demote` |

---

## ANSI Escape Codes Used

| Code | Meaning | Where Used |
|------|---------|------------|
| `\r` | Carriage return — move cursor to start of line | `client/client.go:302` |
| `\033[K` | Erase from cursor to end of line | `client/client.go:303` |
| `\b \b` | Backspace visual effect (back, space, back) | `client/client.go:268` |
| `\r\n` | Newline (carriage return + line feed) | `client/client.go:272` |

---

## Error Messages Decoded

| Message | When You See It | What It Means |
|---------|----------------|---------------|
| `"Name cannot be empty."` | Empty name entered | User pressed Enter without typing |
| `"Name cannot contain spaces."` | Name has spaces | "John Doe" → not allowed |
| `"Name too long (max 32 characters)."` | Name exceeds limit | Self-explanatory |
| `"Name must contain only printable characters."` | Control chars in name | Tab, escape, etc. in the name |
| `"Name is already taken."` | Duplicate name | Another user already has that name |
| `"Name 'Server' is reserved."` | Used reserved name | "Server" is the only reserved name |
| `"You are muted."` | Muted user tries to chat | Admin muted them |
| `"Message too long (max 2048 characters)."` | Message exceeds limit | Self-explanatory |
| `"Insufficient privileges."` | User tries admin command | Regular user tried `/kick` etc. |
| `"User 'X' not found. Use /list..."` | Invalid target | Target name doesn't exist |
| `"You are banned from this server."` | Banned IP reconnects | IP is on the ban list |
| `"You are temporarily blocked."` | Kicked IP reconnects too soon | 5-minute cooldown not expired |
| `"Server is shutting down. Goodbye!"` | Ctrl+C pressed | Server is closing |
| `"Connection unstable..."` | Heartbeat probe was slow | Network is degraded but alive |

---

## Glossary Complete!

**You now have:**
- Essential vocabulary for reading Go code
- A decoder ring for every abbreviation and magic number
- Quick reference for error messages and message types
- Function naming patterns so you can guess what a function does before reading it

**Congratulations!** You're ready to:
1. Navigate any file in this codebase with confidence
2. Understand WHY the code is written the way it is
3. Debug issues by tracing data flow and checking gotchas
4. Communicate about the code using the correct terminology
5. Add features following the established patterns

**The journey from "vibecoding" to understanding is complete.**
