# Data Flow

## The Factory Assembly Line

Think of this program like a factory:
1. **Raw materials arrive** — someone types text into their terminal
2. **Workers inspect and transform** — the server parses, validates, and routes
3. **Finished product ships out** — formatted messages appear on everyone's screen

This chapter traces every important flow through the code, step by step.

---

## How to Read These Diagrams

- **Boxes** = things (data, components, files)
- **Arrows** = movement / transformation
- **Labels on arrows** = what's happening at that step
- **Dashed lines** = optional or conditional paths

---

## Flow 1: A Chat Message (The Main Path)

This is the most common thing that happens — someone types a message and everyone sees it.

```
  Alice's Terminal                  Server                    Bob's Terminal
  ──────────────                  ──────                    ──────────────
       │                             │                             │
  1. Types "hello"                   │                             │
  ─ ─ key by key ─ ─ ─►             │                             │
       │                             │                             │
  2. Presses Enter                   │                             │
  ────── TCP bytes ─────►            │                             │
       │                    3. ReadLineInteractive()               │
       │                       returns "hello"                     │
       │                             │                             │
       │                    4. ParseCommand("hello")               │
       │                       → not a command                     │
       │                             │                             │
       │                    5. handleChatMessage()                 │
       │                       a. Checks: empty? too long? muted? │
       │                       b. Creates Message struct           │
       │                       c. recordEvent() → log + history   │
       │                       d. Broadcast() to all except Alice  │
       │                       e. SendPrompt() to Alice            │
       │                             │                             │
       │                             ├──── msg via channel ───────►│
       │                             │                    6. writeLoop()
       │                             │                       writes to Bob's
       │                             │                       TCP connection
       │◄─── prompt via channel ─────┤                             │
  7. Sees new prompt                 │                 7. Sees the message
```

### The Story in Plain English

1. Alice types "hello" in her terminal. Each keystroke is read individually by `ReadLineInteractive()` and echoed back.
2. When Alice presses Enter, `ReadLineInteractive()` returns the full line `"hello"`.
3. The handler calls `ParseCommand("hello")`. Since it doesn't start with `/`, it's not a command.
4. `handleChatMessage()` is called. It checks: is the message empty? Too long (>2048 chars)? Is Alice muted?
5. A `Message` struct is created with the timestamp, sender ("Alice"), content ("hello"), and type (`MsgChat`).
6. `recordEvent()` saves it to both in-memory history AND the daily log file.
7. `Broadcast()` sends the formatted message to every client except Alice.
8. Alice gets a new prompt. Bob sees: `[2026-02-24 14:30:05][Alice]:hello`

### Data Transformations (Step by Step)

**STEP 1:** Raw input (bytes from TCP)
```
h e l l o \r \n
```

**STEP 2:** After ReadLineInteractive() → string
```
"hello"
```

**STEP 3:** After Message creation → struct
```go
Message{
    Timestamp: 2026-02-24 14:30:05,
    Sender:    "Alice",
    Content:   "hello",
    Type:      MsgChat,
}
```

**STEP 4:** After Display() → formatted string for clients
```
[2026-02-24 14:30:05][Alice]:hello
```

**STEP 5:** After FormatLogLine() → log file format
```
[2026-02-24 14:30:05] CHAT [Alice]:hello
```

---

## Flow 2: A New Client Connecting

```
  New User's Terminal              Server
  ───────────────────              ──────
       │                             │
  1. nc localhost 8989               │
  ────── TCP connect ──────►         │
       │                    2. listener.Accept()
       │                    3. go handleConnection(conn)
       │                             │
       │                    4. IsIPBlocked()?
       │                       NO → continue
       │                             │
       │                    5. checkOrQueue()
       │                       Under 10 clients → proceed
       │                             │
       │◄── banner + prompt ─────────│
  6. Sees penguin art                │
     "[ENTER YOUR NAME]:"           │
       │                             │
  7. Types "Alice"                   │
  ────── name ─────────────►         │
       │                    8. ValidateName("Alice")
       │                       → OK
       │                    9. RegisterClient()
       │                       → adds to client map
       │                             │
       │                   10. IsKnownAdmin("Alice")?
       │                       if yes → restore admin
       │                             │
       │◄── chat history ────────────│
       │◄── prompt ──────────────────│
       │                             │
       │                   11. Broadcast join to others
       │                   12. recordEvent(join)
       │                   13. go startHeartbeat()
       │                             │
       │                   14. Enter message loop ──►
```

### The Story in Plain English

1. User runs `nc localhost 8989`, opening a TCP connection.
2. The server's `acceptLoop()` accepts the connection.
3. A new goroutine is spawned for this connection.
4. First check: is this IP kicked or banned? If yes, send rejection and close immediately.
5. Second check: is the server full? If yes, offer a queue position.
6. Send the welcome banner (penguin ASCII art) and the name prompt.
7. Read lines in a loop until the user provides a valid, unique name.
8. Validate: non-empty, no spaces, max 32 chars, printable ASCII only, not "Server".
9. Register: atomically check capacity again, check uniqueness, add to client map.
10. Check `admins.json` — if this name is a known admin, restore their powers.
11. Send all previous messages (history) so the user catches up.
12. Broadcast "Alice has joined our chat..." to everyone else.
13. Start a heartbeat goroutine to detect if Alice disconnects unexpectedly.
14. Enter the main message loop — reading and processing input forever.

---

## Flow 3: Command Dispatch

```
  Alice types "/kick Bob"
       │
       ▼
  ReadLineInteractive() → "/kick Bob"
       │
       ▼
  ParseCommand("/kick Bob")
       │
       ├── name = "kick"
       ├── args = "Bob"
       └── isCommand = true
       │
       ▼
  dispatchCommand(client, "kick", "Bob")
       │
       ▼
  Look up "kick" in cmd.Commands
       │
       ├── Found: CommandDef{MinPriv: PrivAdmin}
       │
       ▼
  Check privilege: is Alice an admin?
       │
       ├── NO  → "Insufficient privileges.\n"
       │
       └── YES → cmdKick(c, "Bob")
                    │
                    ├── Find Bob in client map
                    ├── Set Bob's disconnect reason to "kicked"
                    ├── Remove Bob from client map
                    ├── Record moderation event (history + log)
                    ├── Broadcast "Bob was kicked by Alice"
                    ├── Send "You have been kicked" to Bob
                    ├── Close Bob's connection
                    ├── Add Bob's IP to 5-minute cooldown
                    └── Admit next person from queue (if any)
```

---

## Flow 4: The Write Path (How Messages Reach the Screen)

This is subtle but important. The server never writes directly to a client's TCP connection (with one exception: heartbeat probes). Instead, everything goes through a **channel → write goroutine** pipeline.

```
  Any goroutine               Client's channel            Client's writeLoop
  (handler, broadcast,        (msgChan)                   (dedicated goroutine)
   operator, heartbeat)
       │                             │                          │
  c.Send("text")                     │                          │
       │                             │                          │
  enqueue(writeMsg{                  │                          │
    data: "text",                    │                          │
    msgType: wmMessage               │                          │
  })                                 │                          │
       │                             │                          │
       ├──── put in channel ────────►│                          │
       │                             │                          │
       │                             ├──── read from channel ──►│
       │                             │                          │
       │                             │                   Is echoMode on?
       │                             │                     │
       │                             │              ┌──────┴──────┐
       │                             │              │ NO          │ YES
       │                             │              │ Raw write   │ Clear line,
       │                             │              │ to Conn     │ write msg,
       │                             │              │             │ redraw prompt
       │                             │              │             │ + partial input
       │                             │              └─────────────┘
```

### Why Not Write Directly?

Multiple goroutines write to the same client (the handler, broadcast from other handlers, the operator, heartbeat warnings). If they all wrote directly to the TCP connection, bytes could interleave and produce garbage.

The channel + single writer goroutine pattern guarantees that all output is **serialized** — one message at a time, in order.

---

## Flow 5: The Queue (When the Server is Full)

```
  11th Client                      Server                    10 Active Clients
  ───────────                      ──────                    ─────────────────
       │                             │                              │
  1. Connects                        │                              │
       │                    2. checkOrQueue()                       │
       │                       10 clients → FULL                   │
       │                             │                              │
       │◄── "Chat is full.          │                              │
       │     You are #1 in          │                              │
       │     the queue." ───────────│                              │
       │                             │                              │
  3. Types "yes"                     │                              │
       │                             │                              │
       ├─── waitForAdmission() ─────►│                              │
       │    (BLOCKS here)            │                              │
       │                             │         4. Someone disconnects
       │                             │◄─────────────────────────────│
       │                             │                              │
       │                    5. admitFromQueue()                     │
       │                       closes entry.admit channel           │
       │                             │                              │
       │◄── Welcome banner! ─────────│                              │
       │                             │                              │
  6. Normal onboarding               │                              │
     continues                       │                              │
```

---

## Flow 6: Heartbeat (Ghost Client Detection)

```
  Time passes...               Heartbeat Goroutine          Client Connection
  ──────────────               ───────────────────          ─────────────────
       │                              │                            │
  Every 10 seconds ──────────►        │                            │
       │                     Has client sent data                  │
       │                     recently?                             │
       │                       │                                   │
       │                  ┌────┴────┐                              │
       │                  │ YES     │ NO                           │
       │                  │ Skip    │ Send null byte probe         │
       │                  │ probe   │        │                     │
       │                  │         │        ├── Write \x00 ──────►│
       │                  │         │        │                     │
       │                  │         │   ┌────┴────┐                │
       │                  │         │   │ Success │ Timeout (5s)   │
       │                  │         │   │ Healthy │ Dead client    │
       │                  │         │   │         │ → Close()      │
       │                  │         │   └─────────┘                │
       │                  └─────────┘                              │
```

### Why Null Bytes?

The heartbeat sends `\x00` (a null byte) because:
- Terminal emulators silently ignore null bytes — the user never sees it
- If the write succeeds, the TCP connection is alive
- If the write fails or times out, the client is gone (crashed, lost network, etc.)

---

## Flow 7: Logging and Recovery

```
  During Runtime                                    On Restart
  ──────────────                                    ──────────

  recordEvent(msg)                              RecoverHistory()
       │                                              │
       ├── AddHistory(msg)                       Open today's log file
       │   (in-memory slice)                          │
       │                                         Read line by line
       ├── Logger.Log(msg)                            │
       │        │                                ParseLogLine(line)
       │        ▼                                     │
       │   msg.FormatLogLine()                   Reconstruct Message
       │   → "[2026-02-24 14:30:05] CHAT [Alice]:hello"    │
       │        │                                Skip MsgServerEvent
       │        ▼                                     │
       │   Write to logs/chat_2026-02-24.log     AddHistory(msg)
       │                                         (rebuilds in-memory history)
```

### The Story in Plain English

**During runtime:** Every event (chat, join, leave, kick, etc.) is both added to an in-memory list AND written to a log file. The in-memory list is what new clients receive as "history." The log file is the permanent record.

**On restart:** The server reads today's log file and reconstructs the in-memory history. This means if the server crashes and restarts, clients still see all the messages from today.

---

## The Complete Picture

```
                         ┌─────────────────────────────┐
                         │         main.go             │
                         │  port → Server → Logger     │
                         │  Ctrl+C → Shutdown          │
                         └─────────────┬───────────────┘
                                       │
                         ┌─────────────▼───────────────┐
                         │       Server.Start()        │
                         │  LoadAdmins() ← admins.json │
                         │  RecoverHistory() ← log file│
                         │  startMidnightWatcher()     │
                         │  acceptLoop()               │
                         └─────────────┬───────────────┘
                                       │
              ┌────────────────────────┼────────────────────────┐
              │                        │                        │
    ┌─────────▼─────────┐   ┌─────────▼─────────┐   ┌─────────▼─────────┐
    │   Connection 1    │   │   Connection 2    │   │   Connection N    │
    │                   │   │                   │   │                   │
    │  handleConnection │   │  handleConnection │   │  handleConnection │
    │       │           │   │       │           │   │       │           │
    │  ┌────▼────┐      │   │  ┌────▼────┐      │   │  ┌────▼────┐      │
    │  │writeLoop│      │   │  │writeLoop│      │   │  │writeLoop│      │
    │  └─────────┘      │   │  └─────────┘      │   │  └─────────┘      │
    │  ┌─────────┐      │   │  ┌─────────┐      │   │  ┌─────────┐      │
    │  │heartbeat│      │   │  │heartbeat│      │   │  │heartbeat│      │
    │  └─────────┘      │   │  └─────────┘      │   │  └─────────┘      │
    └───────────────────┘   └───────────────────┘   └───────────────────┘
              │                        │                        │
              └────────────────────────┼────────────────────────┘
                                       │
                         ┌─────────────▼───────────────┐
                         │       Shared State          │
                         │  clients map (mutex)        │
                         │  history slice (mutex)      │
                         │  kickedIPs map (mutex)      │
                         │  bannedIPs map (mutex)      │
                         │  admins map (mutex)         │
                         │  queue slice (mutex)        │
                         └─────────────┬───────────────┘
                                       │
                         ┌─────────────▼───────────────┐
                         │        Logger               │
                         │  logs/chat_YYYY-MM-DD.log   │
                         └─────────────────────────────┘
```

---

## What to Read Next

Now that you see how data flows, learn the recurring patterns that shape this design: [03 - Patterns](03-patterns.md)
