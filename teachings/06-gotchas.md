# Gotchas

## Why This Chapter Exists

Every codebase has "gotchas" — things that are obvious AFTER you know them, but invisible until you hit the bug. Each gotcha in this file represents a **real bug** that someone could easily introduce while modifying this code.

Understanding these isn't just about avoiding mistakes — it's about developing the **defensive thinking** that separates robust code from fragile code.

---

## Gotcha 1: Closing a Channel Twice = Panic

**The bug:** Go panics (crashes) if you close a channel that's already closed. In this codebase, the `done` channel on each client is used as a shutdown signal. Multiple goroutines (handler, heartbeat, operator kick) might all try to close a client simultaneously.

```go
// WRONG: closing done twice panics
func (c *Client) Close() {
    close(c.done)     // First call: fine
    c.Conn.Close()
}
// ... later, another goroutine also calls Close()
// close(c.done)     // PANIC: close of closed channel
```

```go
// RIGHT: sync.Once ensures it only runs once
func (c *Client) Close() {
    c.closeOnce.Do(func() {
        close(c.done)     // Guaranteed: only executes once
        c.Conn.Close()
    })
}
```

**How to spot this bug:** The program crashes with `panic: close of closed channel`. The stack trace will show two goroutines racing to close the same channel.

**Code location:** `client/client.go:119-124`

---

## Gotcha 2: TOCTOU Race in Registration

**The bug:** TOCTOU stands for "Time-Of-Check-To-Time-Of-Use." If you check "is this name taken?" and then add the name in two separate lock acquisitions, another goroutine can sneak in between and register the same name.

```go
// WRONG: check and add are not atomic
s.mu.RLock()
_, exists := s.clients[name]
s.mu.RUnlock()
// ← RIGHT HERE: another goroutine registers the same name
s.mu.Lock()
s.clients[name] = c   // BUG: name might already be taken
s.mu.Unlock()
```

```go
// RIGHT: check and add in one lock acquisition
s.mu.Lock()
defer s.mu.Unlock()
if _, exists := s.clients[name]; exists {
    return errors.New("name taken")
}
s.clients[name] = c   // Safe: no one else can modify between check and add
```

**How to spot this bug:** Two clients with the same name appear simultaneously. It only happens under load (multiple clients connecting at once), making it hard to reproduce.

**Code location:** `server/server.go:195-213`

---

## Gotcha 3: Writing to a TCP Connection from Multiple Goroutines

**The bug:** Multiple goroutines sending data to the same TCP connection can interleave bytes, producing garbled output. This is not protected by Go's runtime — TCP writes are not atomic.

```go
// WRONG: two goroutines write directly
// Goroutine A: c.Conn.Write([]byte("Hello from Alice\n"))
// Goroutine B: c.Conn.Write([]byte("Hello from Bob\n"))
// Result: "Hello froHello from m Alice\nBob\n"  (interleaved garbage)
```

```go
// RIGHT: all writes go through a channel → single writer goroutine
c.Send("Hello from Alice\n")   // enqueue → channel → writeLoop → Conn.Write
c.Send("Hello from Bob\n")     // writeLoop processes one at a time
```

**How to spot this bug:** Garbled text on the client's terminal. Characters from different messages appear mixed together. It's intermittent and timing-dependent.

**Code location:** `client/client.go:232-278` (writeLoop is the sole writer)

**Exception:** The heartbeat writes `\x00` directly to `Conn` — this is safe because a single null byte cannot interleave with itself, and the writeLoop never writes null bytes.

---

## Gotcha 4: `\r\n` vs `\r` vs `\n` Line Endings

**The bug:** Different terminals send different line endings when the user presses Enter:
- Linux/macOS: `\n` (newline)
- Windows: `\r\n` (carriage return + newline)
- Some raw terminals: `\r` (carriage return only)

If you don't handle all three, some users' input won't be recognized.

```go
// WRONG: only handling \n
if b == '\n' {
    return string(line), nil
}
// Users with \r-only terminals can never submit input!

// RIGHT: handle \r, \n, and \r\n
case b == '\r':
    c.skipLF = true          // Flag: if next byte is \n, skip it
    return string(line), nil // \r ends the line
case b == '\n':
    return string(line), nil // \n ends the line
// The skipLF flag prevents \r\n from producing TWO line submissions
```

**How to spot this bug:** Users connecting from certain terminals (especially Windows `telnet`) either can't press Enter, or their Enter produces two submissions.

**Code location:** `client/client.go:342-356`

---

## Gotcha 5: Forgetting to Admit from Queue After Removal

**The bug:** Every time a client leaves the active client map (disconnect, kick, ban), a slot opens up. If you forget to call `admitFromQueue()`, queued clients wait forever even though there's space.

```go
// WRONG: removing a client but forgetting queue admission
s.RemoveClient(username)
s.Broadcast(models.FormatLeave(username)+"\n", username)
// Queue users wait forever!

// RIGHT: always admit after removing
s.RemoveClient(username)
s.Broadcast(models.FormatLeave(username)+"\n", username)
s.admitFromQueue()   // Let the next queued person in
```

**How to spot this bug:** Users in the queue never get admitted even when active clients disconnect. The queue length grows but never shrinks.

**Code locations:**
- `server/handler.go:146` — deferred cleanup calls `admitFromQueue()`
- `server/handler.go:580` — `/kick` calls `admitFromQueue()`
- `server/handler.go:647-648` — `/ban` calls `admitFromQueue()` for each slot opened

---

## Gotcha 6: The Deferred Cleanup Double-Broadcast

**The bug:** When a client is kicked or banned, the moderation command already broadcasts the event and removes the client from the map. When the handler function returns, the deferred cleanup would normally ALSO broadcast a leave event and remove from the map — causing a double announcement and a map deletion of a key that's already gone.

```go
// The defer in handleConnection:
defer func() {
    username := c.Username
    switch c.GetDisconnectReason() {
    case "kicked", "banned":
        // DO NOTHING — the moderation command already handled it
    default:
        s.RemoveClient(username)
        s.recordEvent(leaveMsg)
        s.Broadcast(...)
    }
    s.admitFromQueue()
    c.Close()
}()
```

**How to spot this bug:** A kicked user would appear twice in the chat: "Bob was kicked by Alice" followed by "Bob has left our chat..." — confusing for everyone.

**Code location:** `server/handler.go:126-148`

---

## Gotcha 7: Blocking the Server with a Slow Client

**The bug:** If `Broadcast()` waited for each client to consume the message (blocking channel send), a single slow client would freeze the entire broadcast — nobody else receives messages until the slow client catches up or disconnects.

```go
// WRONG: blocking send in broadcast
for _, c := range s.clients {
    c.msgChan <- msg   // Blocks if channel full! Freezes entire server!
}

// RIGHT: non-blocking send with drop
select {
case c.msgChan <- m:   // Success
case <-c.done:         // Client closing
default:               // Full — drop for THIS client only
}
```

**How to spot this bug:** The server freezes. No one can send or receive messages. After the slow client eventually times out or disconnects, everything resumes. Under high traffic, this could cascade into all clients timing out.

**Code location:** `client/client.go:92-104`

---

## Gotcha 8: Reusing a Scanner After Read Deadline

**The bug:** When a queued client is admitted, the server cancels the monitoring goroutine by setting a read deadline on the connection (`c.Conn.SetReadDeadline(time.Now())`). This causes the scanner to enter an error state. If you reuse the same scanner, all subsequent reads fail.

```go
// WRONG: reusing scanner after deadline
c.Conn.SetReadDeadline(time.Now())    // Cancel the monitor
<-monitorDone                          // Wait for it to exit
c.Conn.SetReadDeadline(time.Time{})    // Clear the deadline
// c.scanner is in error state! All future ReadLine() calls fail!

// RIGHT: create a fresh scanner
c.Conn.SetReadDeadline(time.Now())
<-monitorDone
c.Conn.SetReadDeadline(time.Time{})
c.ResetScanner()   // Creates a new bufio.Scanner on the same connection
```

**How to spot this bug:** After being admitted from the queue, the client can't type their name — every `ReadLine()` returns an error, and the connection is immediately dropped.

**Code location:** `server/handler.go:282-286` and `client/client.go:144-148`

---

## Gotcha 9: Atomic File Write for Crash Safety

**The bug:** If you write `admins.json` directly and the server crashes mid-write, the file is half-written and corrupt. On restart, `LoadAdmins()` fails and you lose all admin assignments.

```go
// WRONG: direct write (crash = corruption)
os.WriteFile("admins.json", data, 0600)

// RIGHT: write to temp file, then atomic rename
os.WriteFile(".admins.json.tmp", data, 0600)   // Temp file
os.Rename(".admins.json.tmp", "admins.json")    // Atomic swap
```

**How to spot this bug:** After a crash, the server starts with no admins. `LoadAdmins()` prints a warning about corrupt JSON. The `.admins.json.tmp` file might exist as evidence.

**Code location:** `server/server.go:554-593`

---

## Gotcha 10: Nil Logger Calls

**The bug:** The logger might fail to initialize (e.g., can't create the `logs/` directory). If the server continues without a logger (which it does — logging is not critical), every `s.Logger.Log()` call would panic with a nil pointer dereference.

```go
// WRONG: calling methods on nil
var l *Logger = nil
l.Log(msg)   // PANIC: nil pointer dereference

// RIGHT: nil-safe method
func (l *Logger) Log(msg models.Message) {
    if l == nil {     // First thing: check if receiver is nil
        return        // Silently do nothing
    }
    // ... actual logging
}
```

**How to spot this bug:** The server crashes on the first log attempt with `runtime error: invalid memory address or nil pointer dereference`.

**Code location:** `logger/logger.go:34-36` (nil check at the top of `Log()`)

---

## Gotcha 11: Logger Reopening After Shutdown

**The bug:** After `Shutdown()` calls `Logger.Close()`, late-running goroutines might still call `Logger.Log()`. Without protection, `Log()` would call `ensureFile()` which reopens the log file — leaking a file descriptor and potentially writing partial data.

```go
// WRONG: no closed check
func (l *Logger) Log(msg models.Message) {
    l.mu.Lock()
    defer l.mu.Unlock()
    l.ensureFile(date)   // Reopens the file even after Close()!
    l.file.WriteString(line)
}

// RIGHT: check closed flag
func (l *Logger) Log(msg models.Message) {
    l.mu.Lock()
    defer l.mu.Unlock()
    if l.closed {        // Set by Close()
        return           // Don't reopen
    }
    l.ensureFile(date)
    l.file.WriteString(line)
}
```

**How to spot this bug:** Log files appear to be written after the "Server shutting down" entry, or a file descriptor leak shows up in `lsof`.

**Code location:** `logger/logger.go:40-42`

---

## Gotcha 12: NAT Collateral Damage on /ban

**The bug:** When banning a user by IP, you might forget that multiple people can share the same public IP (e.g., in an office or behind a NAT router). Banning one person should also disconnect all other clients from that same IP — otherwise they can keep chatting from a "banned" address.

```go
// WRONG: only ban the target
s.AddBanIP(targetIP)
target.Close()
// Other users from the same IP keep chatting!

// RIGHT: also disconnect same-IP clients
s.AddBanIP(targetIP)
target.Close()
collateral := s.GetClientsByIP(bannedHost, c.Username)
for _, cc := range collateral {
    cc.ForceDisconnectReason("banned")
    s.RemoveClient(cc.Username)
    cc.Close()
}
```

**How to spot this bug:** After banning someone, they reconnect from the same IP under a different name. Or another person behind the same NAT keeps chatting, then gets banned when they try to reconnect.

**Code location:** `server/handler.go:617-637`

---

## Gotcha 13: Insertion Sort Without `sort` Package

**The bug (well, not a bug — a design choice):** This codebase intentionally avoids importing the `sort` package. Wherever sorting is needed (admin list, client list), it uses inline insertion sort. If you add a new feature that needs sorting and import `sort`, you're breaking a project convention.

```go
// Project convention: inline insertion sort
for i := 1; i < len(names); i++ {
    key := names[i]
    j := i - 1
    for j >= 0 && names[j] > key {
        names[j+1] = names[j]
        j--
    }
    names[j+1] = key
}
```

**How to spot this mistake:** A code review catches the new import. The `go.mod` file stays clean (no external dependencies), but the standard library `sort` package would still be a style violation.

**Code locations:** `server/server.go:568-576` (SaveAdmins), `server/server.go:833-841` (operatorCmdList), `server/handler.go:425-433` (cmdList)

---

## Gotcha 14: FormatLogLine / ParseLogLine Mismatch

**The bug:** `FormatLogLine()` and `ParseLogLine()` are a **matched pair**. If you change the format of one without updating the other, history recovery breaks — the server can't read its own log files.

```go
// FormatLogLine produces:
"[2026-02-24 14:30:05] CHAT [Alice]:hello"

// ParseLogLine expects EXACTLY that format:
// - Timestamp in brackets
// - Space after bracket
// - Type keyword (CHAT, JOIN, LEAVE, etc.)
// - Type-specific format after the keyword
```

**How to spot this bug:** After a restart, `RecoverHistory()` prints warnings about "corrupt log lines" — it's reading lines in the new format with the old parser (or vice versa).

**Code locations:** `models/message.go:106-130` (FormatLogLine) and `models/message.go:133-225` (ParseLogLine)

---

## Gotcha 15: `SetDisconnectReason` vs `ForceDisconnectReason`

**The bug:** `SetDisconnectReason` only sets the reason if it's empty (first writer wins). `ForceDisconnectReason` always overwrites. Using the wrong one leads to incorrect leave messages.

```go
// SetDisconnectReason: defensive, first-writer-wins
func (c *Client) SetDisconnectReason(reason string) {
    c.mu.Lock()
    if c.disconnectReason == "" {   // Only set if not already set
        c.disconnectReason = reason
    }
    c.mu.Unlock()
}

// ForceDisconnectReason: for moderation (always overwrites)
func (c *Client) ForceDisconnectReason(reason string) {
    c.mu.Lock()
    c.disconnectReason = reason     // Always overwrite
    c.mu.Unlock()
}
```

**When to use which:**
- `SetDisconnectReason("drop")` — heartbeat/read error detected a dead client. Use the "set" variant because a kick/ban reason might already be there.
- `ForceDisconnectReason("kicked")` — admin is kicking this client. Use "force" because we KNOW the reason, even if a heartbeat race set "drop" first.

**How to spot this bug:** A kicked user's leave message says "drop" instead of "kicked" because the heartbeat detected the closed connection a microsecond before the kick handler set the reason.

**Code locations:** `client/client.go:166-172` (Set) and `client/client.go:182-186` (Force)

---

## The Gotcha Mindset

When reading or writing code, always ask:

1. **"What could go wrong?"** — Edge cases, bad input, timeouts, concurrent access
2. **"What order do things happen?"** — Goroutine scheduling is non-deterministic
3. **"Who else touches this data?"** — If the answer is "multiple goroutines," you need synchronization
4. **"What happens if this fails?"** — Every network call, file operation, and channel send can fail
5. **"What's the cleanup path?"** — `defer` exists for a reason. Resources that are opened must be closed.

This defensive thinking separates robust code from fragile code. It's the difference between "works in testing" and "works in production."

---

## What to Read Next

Get a quick-reference for every term and abbreviation in the codebase: [07 - Glossary](07-glossary.md)
