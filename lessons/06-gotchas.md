# Lesson 06: Gotchas

## Why This Chapter Exists

Every codebase has "gotchas" — things that are obvious AFTER you know them but can trip you up if you don't. Each gotcha here represents a **bug someone could easily introduce**. Understanding them makes you a better debugger.

---

## Gotcha 1: TOCTOU Race in Registration

**The bug:** Checking if a name is available and then inserting it must be atomic. If you split these into two operations, two goroutines can both see the name as available and both insert.

```go
// WRONG — check and insert are separate
func (s *Server) RegisterClient(c *client.Client, name string) error {
    s.mu.RLock()
    _, exists := s.clients[name]
    s.mu.RUnlock()              // Lock released here!
    // WINDOW: another goroutine steals the name RIGHT HERE
    if exists {
        return errors.New("taken")
    }
    s.mu.Lock()
    s.clients[name] = c         // Both goroutines reach here!
    s.mu.Unlock()
    return nil
}

// RIGHT — check and insert under the same lock
func (s *Server) RegisterClient(c *client.Client, name string) error {
    s.mu.Lock()                 // Hold the lock for BOTH operations
    defer s.mu.Unlock()
    if _, exists := s.clients[name]; exists {
        return errors.New("taken")
    }
    s.clients[name] = c         // Only one goroutine can be here
    return nil
}
```

**How to spot this bug:** Two users end up with the same name. One is a "ghost" — registered but not in the client map.

**Code location:** `server/clients.go:30-45`

---

## Gotcha 2: Double-Close Panic

**The bug:** Closing a channel twice in Go causes a panic. If a client's connection drops (heartbeat detects it) at the same moment the handler reads an error, both paths try to close the client.

```go
// WRONG
func (c *Client) Close() {
    close(c.done)       // Second call: PANIC!
    c.Conn.Close()
}

// RIGHT
func (c *Client) Close() {
    c.closeOnce.Do(func() {
        close(c.done)
        c.Conn.Close()
    })
}
```

**How to spot this bug:** Panic stack trace mentioning "close of closed channel."

**Code location:** `client/client.go:120-125`

---

## Gotcha 3: The `\r\n` Double-Submit

**The bug:** Windows terminals send `\r\n` for Enter (two bytes). Without handling, the server processes `\r` as one line submission and `\n` as another, creating a phantom empty message.

```go
// WRONG — no CRLF handling
case b == '\r' || b == '\n':
    return string(line), nil    // Returns line for \r, then empty string for \n

// RIGHT — skip \n after \r
case b == '\r':
    c.skipLF = true             // Next \n will be ignored
    return string(line), nil
```

**How to spot this bug:** Users see empty messages or double prompts. Happens only with certain terminal clients (Windows `netcat`, PuTTY).

**Code location:** `client/client.go:339-344`

---

## Gotcha 4: Blocking Sends Freeze the Server

**The bug:** If `c.Send()` were a blocking call (regular channel send), a single slow client could block the entire broadcast. The goroutine holding `s.mu` would block on the channel send, preventing ALL other operations.

```go
// WRONG — blocking send
func (c *Client) Send(msg string) {
    c.msgChan <- writeMsg{data: msg}  // Blocks if buffer is full!
    // Caller holds s.mu.RLock → other goroutines can't Lock → server freezes
}

// RIGHT — non-blocking with drop
func (c *Client) enqueue(m writeMsg) {
    select {
    case c.msgChan <- m:    // Send if space available
    case <-c.done:          // Client already closed
    default:                // Buffer full → DROP. One client's problem
    }                       //   doesn't become everyone's problem.
}
```

**How to spot this bug:** The entire server hangs. All clients freeze. CPU is idle (goroutines are blocked, not spinning).

**Code location:** `client/client.go:93-105`

---

## Gotcha 5: Moderation Double-Cleanup

**The bug:** When a user is kicked, the kick handler removes them from the client map and broadcasts the leave message. But the handler's `defer` block also removes them and broadcasts. Without coordination, you'd get duplicate leave messages.

```go
// The solution: check disconnectReason in the defer
defer func() {
    switch c.GetDisconnectReason() {
    case "kicked", "banned":
        // Moderation handler already did cleanup — SKIP
    default:
        s.RemoveClient(username)              // Only for normal exits
        s.BroadcastRoom(room, FormatLeave...) // Only for normal exits
    }
}()
```

**How to spot this bug:** Users see "alice was kicked by admin" followed by "alice has left our chat..." Two leave messages for one event.

**Code location:** `server/handler.go:134-158`

---

## Gotcha 6: SetDisconnectReason — First Wins

**The bug:** A user could be kicked and simultaneously have their connection drop. Both events try to set the disconnect reason. The reason must be set atomically, and the first one wins.

```go
// SetDisconnectReason only sets if empty (first caller wins)
func (c *Client) SetDisconnectReason(reason string) {
    c.mu.Lock()
    if c.disconnectReason == "" {   // Only set once!
        c.disconnectReason = reason
    }
    c.mu.Unlock()
}

// ForceDisconnectReason overrides unconditionally (for moderation)
func (c *Client) ForceDisconnectReason(reason string) {
    c.mu.Lock()
    c.disconnectReason = reason     // Always overwrite — moderation takes priority
    c.mu.Unlock()
}
```

**Why two methods?** Normal disconnects use `SetDisconnectReason` (first caller wins). Moderation uses `ForceDisconnectReason` because a kick/ban should always be recorded as the reason, even if a connection drop was detected a millisecond earlier.

**Code location:** `client/client.go:167-187`

---

## Gotcha 7: Heartbeat vs. WriteLoop Conflict

**The bug:** The heartbeat needs to write a null byte to detect dead connections. But the writeLoop is the sole writer to the connection. Using `SetWriteDeadline` from the heartbeat goroutine would interfere with the writeLoop's writes.

```go
// WRONG — SetWriteDeadline affects ALL writes, including writeLoop
func (s *Server) startHeartbeat(c *client.Client) {
    c.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
    c.Conn.Write([]byte{0})                  // writeLoop writes may timeout!
    c.Conn.SetWriteDeadline(time.Time{})     // Clear — but too late
}

// RIGHT — goroutine with timer, no deadline manipulation
go func() {
    _, err := c.Conn.Write([]byte{0})        // This IS a concurrent write,
    probeResult <- err                        // but \x00 is harmless to the stream
}()
select {
case err := <-probeResult: // handle
case <-time.After(timeout): // timed out
}
```

**How to spot this bug:** Legitimate messages fail to send with "deadline exceeded" errors. Clients randomly disconnect with working connections.

**Code location:** `server/server.go:241-246`

---

## Gotcha 8: Queue Monitor Goroutine Cleanup

**The bug:** When a client is queued and waiting for admission, a goroutine monitors the connection for disconnects. When the client IS admitted, this monitoring goroutine must be stopped cleanly, or it will keep reading from the connection.

```go
// In waitForRoomAdmission:
select {
case <-entry.admit:
    c.Conn.SetReadDeadline(time.Now())  // Force the monitor's ReadLine to error
    <-monitorDone                       // Wait for the monitor to exit
    c.Conn.SetReadDeadline(time.Time{}) // Clear the deadline
    c.ResetScanner()                    // Create fresh scanner (old one is in error state)
    return true
}
```

**How to spot this bug:** After being admitted from the queue, the client's first message is eaten by the stale monitoring goroutine. Or the scanner is in an error state and all reads fail.

**Code location:** `server/handler.go:317-323`

---

## Gotcha 9: Partial Write Glitches

**The bug:** In echo mode, if the server writes the incoming message and the prompt redraw as separate `Conn.Write` calls, the user might see a momentary glitch — the line clears, the message appears, then a brief pause before the prompt reappears.

```go
// WRONG — multiple writes = possible visual glitch
func (c *Client) writeWithContinuity(msg string) {
    c.Conn.Write([]byte("\r\033[K"))    // Clear line
    c.Conn.Write([]byte(msg))           // Write message
    // USER SEES: blank line here for a moment!
    c.Conn.Write([]byte(c.prompt))      // Redraw prompt
    c.Conn.Write([]byte(c.inputBuf))    // Redraw partial input
}

// RIGHT — single write = atomic visual update
func (c *Client) writeWithContinuity(msg string) {
    buf := make([]byte, 0, size)
    buf = append(buf, '\r')
    buf = append(buf, "\033[K"...)
    buf = append(buf, msg...)
    buf = append(buf, c.prompt...)
    buf = append(buf, c.inputBuf...)
    c.Conn.Write(buf)                   // One write — no flicker
}
```

**How to spot this bug:** The terminal flickers or shows a blank prompt momentarily when messages arrive while typing.

**Code location:** `client/client.go:282-311`

---

## Gotcha 10: Logger Reopening After Shutdown

**The bug:** After `Shutdown()` calls `Logger.Close()`, a late goroutine (e.g., a handler's defer running the leave event) might call `Logger.Log()`. Without the `closed` flag, this would reopen the log file — potentially writing to a file that should be finalized.

```go
func (l *Logger) Log(msg models.Message) {
    if l == nil { return }
    l.mu.Lock()
    defer l.mu.Unlock()
    if l.closed { return }          // Prevents reopening after shutdown
    l.ensureFile(date)              // Would reopen the file without the check!
    l.file.WriteString(line)
}
```

**How to spot this bug:** Log files contain entries timestamped AFTER the "Server shutting down" line. Harmless but confusing.

**Code location:** `logger/logger.go:40-42`

---

## Gotcha 11: Bash `/dev/tcp` Line Buffering

**The bug:** When connecting with `exec 3<>/dev/tcp/localhost/8989; cat <&3 & cat >&3`, the server's echo mode (`ReadLineInteractive`) expects byte-by-byte input. But `cat >&3` is line-buffered by the shell — characters aren't sent until the user presses Enter. This means:

1. The server sends a prompt and waits for character echoes, but no bytes arrive while the user types
2. When Enter is pressed, all bytes arrive at once — the server processes them correctly, but the real-time echo experience is lost
3. `writeWithContinuity` still works (it redraws prompt + partial input), but with line-buffered input the partial input is always empty until the full line arrives

```
NETCAT (byte-by-byte):
  User types 'h' → server receives 'h' → echoes 'h' back → user sees 'h'
  User types 'i' → server receives 'i' → echoes 'i' back → user sees 'hi'

BASH /dev/tcp (line-buffered):
  User types 'hi' locally → nothing sent yet → user sees local 'hi'
  User presses Enter → 'hi\n' arrives all at once → server processes 'hi'
```

**This is NOT a bug in the server** — it's a property of how `cat` buffers terminal input. The server handles both cases correctly. Netcat sends each keystroke immediately (raw socket mode), while `cat` collects a full line first.

**How to spot this:** User reports "I can't see other people's messages while I'm typing" when using the bash method. The messages actually arrive, but since `cat >&3` holds the terminal, the background `cat <&3` output gets interleaved awkwardly.

---

## The Gotcha Mindset

When reading or writing code, always ask:

1. **"What could go wrong?"** — Edge cases, bad input, timeouts, concurrent access
2. **"What order do things happen?"** — Sequence matters. Lock A before B, check before insert.
3. **"What happens if this runs twice?"** — Idempotency. Is double-close safe? Double-insert?
4. **"What happens at boundaries?"** — Midnight (date change), max capacity, empty rooms, shutdown

This defensive thinking separates robust code from fragile code. Every gotcha in this chapter was once a real bug that was found and fixed.

---

## What's Next

The final lesson is the **glossary** — a quick reference for all the terms, abbreviations, and naming conventions used in this codebase.
