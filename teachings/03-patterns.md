# Patterns

## Why Learn Patterns?

A pattern is a **proven solution to a common problem**. Like cooking techniques: you don't reinvent "sauteing" every time — you learn the technique once and apply it everywhere.

Learning patterns helps you:
1. **Read code faster** — "Oh, this is the single-writer pattern"
2. **Write better code** — Use battle-tested solutions instead of inventing fragile ones
3. **Communicate** — "Let's use the producer-consumer pattern here"
4. **Understand WHY** — Not just WHAT the code does, but why it's structured this way

---

## Pattern 1: Single Writer per Resource

### The Problem (Plain English)
Multiple goroutines need to send data to the same TCP connection. If they all write directly, their bytes interleave and produce corrupted output (imagine two people writing on the same whiteboard simultaneously).

### The Solution (The Pattern)
Funnel ALL writes through a single goroutine that owns the connection. Other goroutines send messages via a channel; the writer goroutine is the only one that touches `Conn.Write()`.

### The Code

```go
// WRONG way: multiple goroutines write directly
func (s *Server) Broadcast(msg string) {
    for _, c := range s.clients {
        c.Conn.Write([]byte(msg))  // DANGER: multiple goroutines call this
    }
}

// RIGHT way: send through a channel, one writer goroutine drains it
func (c *Client) Send(msg string) {
    c.enqueue(writeMsg{data: msg, msgType: wmMessage})  // Put in channel
}

func (c *Client) writeLoop() {   // Only this goroutine writes to Conn
    for {
        select {
        case msg := <-c.msgChan:
            c.Conn.Write([]byte(msg.data))   // Single writer
        case <-c.done:
            return
        }
    }
}
```

### Real-World Analogy
- **Bad:** Everyone in the office shouts announcements at the same time (chaos)
- **Good:** Everyone writes announcements on slips of paper, drops them in a box, and one person reads them out one at a time (orderly)

### Code Location
`client/client.go:232-278` — the `writeLoop` goroutine

---

## Pattern 2: Signal Channels (Done / Quit)

### The Problem (Plain English)
Goroutines run independently — how do you tell them "stop what you're doing" cleanly? You can't just kill them (Go doesn't have that). You need a way to signal "please stop."

### The Solution (The Pattern)
Create a channel. When you want to signal "stop," close the channel. Every goroutine that's listening on it immediately knows.

### The Code

```go
// WRONG way: using a boolean flag (race condition!)
type Client struct {
    stopped bool
}
func (c *Client) writeLoop() {
    for !c.stopped {   // Another goroutine sets stopped=true
        // ... BUG: checking a bool without a lock is a data race
    }
}

// RIGHT way: use a channel as a signal
type Client struct {
    done chan struct{}    // Empty struct = zero memory, pure signal
}
func (c *Client) Close() {
    close(c.done)        // Closing a channel "broadcasts" to all receivers
}
func (c *Client) writeLoop() {
    select {
    case msg := <-c.msgChan:
        // handle message
    case <-c.done:       // Immediately unblocks when closed
        return
    }
}
```

### Why `chan struct{}` Instead of `chan bool`?

`struct{}` is an empty type — it takes zero bytes of memory. The channel carries no data, only a signal. `close(done)` is the signal. This is a Go idiom: channels of empty struct = pure signals.

### Real-World Analogy
- **Bad:** Checking if the "open" sign is flipped every few seconds (polling)
- **Good:** A fire alarm — when it goes off, everyone hears it instantly (signal channel)

### Code Location
- `client/client.go:46` — `done chan struct{}` (per-client shutdown)
- `server/server.go:40` — `quit chan struct{}` (server-wide shutdown)
- `server/server.go:28` — `admit chan struct{}` (queue admission signal)

---

## Pattern 3: Lock-and-Copy for Iteration

### The Problem (Plain English)
You need to iterate over the client map (e.g., to broadcast a message), but other goroutines might add or remove clients during iteration. Go maps are not safe for concurrent access.

### The Solution (The Pattern)
Lock the map, copy the data you need, unlock immediately, then work with the copy. This minimizes the time the lock is held.

### The Code

```go
// WRONG way: holding the lock during the entire broadcast (blocking everyone else)
func (s *Server) Broadcast(msg string) {
    s.mu.RLock()
    for _, c := range s.clients {
        c.Send(msg)  // This could block! Other goroutines are frozen out.
    }
    s.mu.RUnlock()
}

// In this codebase, Broadcast IS inside the lock, but Send() is non-blocking
// (it uses a channel with a "drop if full" policy). This is safe because
// Send() never blocks.
func (s *Server) Broadcast(msg string, exclude string) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    for name, c := range s.clients {
        if name != exclude {
            c.Send(msg)  // Non-blocking channel send — safe under lock
        }
    }
}
```

For cases where you DO need to do work after copying:

```go
// GetHistory: copy under lock, return the copy
func (s *Server) GetHistory() []models.Message {
    s.mu.RLock()
    defer s.mu.RUnlock()
    out := make([]models.Message, len(s.history))
    copy(out, s.history)  // Copy the slice
    return out            // Caller works with the copy, no lock needed
}
```

### Real-World Analogy
- **Bad:** Photocopying documents while holding the filing cabinet open (blocking everyone)
- **Good:** Grab the documents, close the cabinet, THEN photocopy at your leisure

### Code Location
- `server/server.go:286-292` — `GetHistory()` copies under lock
- `server/server.go:311-319` — `Broadcast()` iterates under read lock

---

## Pattern 4: Atomic Registration (Check-Then-Act Under Lock)

### The Problem (Plain English)
Registering a new client requires three checks: (1) server not full, (2) name not taken, (3) name not reserved. If these checks happen without a lock, another goroutine could register the same name between check 2 and the actual registration.

This is called a **TOCTOU race** (Time-Of-Check-To-Time-Of-Use).

### The Solution (The Pattern)
Do ALL checks AND the mutation inside a single lock acquisition.

### The Code

```go
// WRONG way: checking then adding without holding the lock continuously
func (s *Server) RegisterClient(c *Client, name string) error {
    s.mu.RLock()
    if _, exists := s.clients[name]; exists {
        s.mu.RUnlock()
        return errors.New("name taken")
    }
    s.mu.RUnlock()
    // BUG: another goroutine could register "name" RIGHT HERE
    s.mu.Lock()
    s.clients[name] = c   // Oops — name might already be taken now
    s.mu.Unlock()
    return nil
}

// RIGHT way: check and add in one atomic operation
func (s *Server) RegisterClient(c *Client, name string) error {
    s.mu.Lock()                              // One lock acquisition
    defer s.mu.Unlock()
    if len(s.clients) >= MaxActiveClients {   // Check 1: capacity
        return errCapacityFull
    }
    if _, exists := s.clients[name]; exists { // Check 2: uniqueness
        return errors.New("name taken")
    }
    if s.reservedNames[name] {                // Check 3: reserved
        return errors.New("name reserved")
    }
    s.clients[name] = c                      // Mutation — guaranteed safe
    return nil
}
```

### Real-World Analogy
- **Bad:** Checking if a hotel room is available, walking away to get your luggage, then returning to find someone else took it
- **Good:** Checking availability and putting your name on the door in one motion, with the front desk locked to other guests

### Code Location
`server/server.go:195-213` — `RegisterClient()`

---

## Pattern 5: Graceful Shutdown with Timeout

### The Problem (Plain English)
When the server stops, you want to give clients a chance to disconnect cleanly. But you can't wait forever — some clients might be unresponsive.

### The Solution (The Pattern)
1. Signal "we're shutting down" (close the quit channel)
2. Stop accepting new connections
3. Tell everyone "goodbye"
4. Wait up to N seconds for them to leave
5. Force-close anyone still connected

### The Code

```go
// server/server.go:105-157
func (s *Server) Shutdown() {
    s.shutdownOnce.Do(func() {        // Only run once, even if called multiple times
        close(s.quit)                  // 1. Signal all goroutines

        s.listener.Close()            // 2. Stop accepting

        for c := range s.allClients { // 3. Say goodbye
            c.Send("Server is shutting down. Goodbye!\n")
        }

        // 4. Wait with timeout
        deadline := time.Now().Add(5 * time.Second)
        for time.Now().Before(deadline) {
            if len(s.allClients) == 0 {
                break                  // Everyone left voluntarily
            }
            time.Sleep(50 * time.Millisecond)
        }

        // 5. Force-close stragglers
        for c := range s.allClients {
            c.Close()
        }
    })
}
```

### Real-World Analogy
A bar at closing time:
1. Turn on the lights and announce "last call"
2. Lock the door (no new customers)
3. Wait 15 minutes for people to finish drinks and leave
4. Anyone still sitting gets asked to leave (force-close)

### Code Location
`server/server.go:105-157` — `Shutdown()`

---

## Pattern 6: Atomic File Write (Write-Then-Rename)

### The Problem (Plain English)
If the server crashes while writing `admins.json`, the file could be half-written and corrupted. On restart, the admin list would be lost.

### The Solution (The Pattern)
Write to a temporary file first, then rename it to the real name. Renaming is atomic on most filesystems — the file is either the old version or the new version, never a corrupt half-version.

### The Code

```go
// WRONG way: writing directly (crash = corruption)
os.WriteFile("admins.json", data, 0600)  // If crash happens mid-write: corrupt file

// RIGHT way: write to temp, then atomic rename
tmpFile := ".admins.json.tmp"
os.WriteFile(tmpFile, data, 0600)   // Write to temporary file
os.Rename(tmpFile, "admins.json")   // Atomic swap — old file replaced instantly
```

### Real-World Analogy
- **Bad:** Erasing a whiteboard and rewriting it (someone walks in mid-erase and sees nothing)
- **Good:** Writing the new version on a second whiteboard, then swapping the two boards instantly

### Code Location
`server/server.go:554-593` — `SaveAdmins()`

---

## Pattern 7: Non-Blocking Send with Drop

### The Problem (Plain English)
If a client's write channel is full (they're slow, or their network is congested), sending a message to them would block the entire server — preventing messages from reaching other clients.

### The Solution (The Pattern)
Use a `select` with a `default` case. If the channel accepts the message, great. If it's full, drop the message for THIS client (they'll miss one message) rather than blocking everyone.

### The Code

```go
// WRONG way: blocking send (one slow client freezes the server)
c.msgChan <- m   // Blocks if channel is full

// RIGHT way: non-blocking send with drop
select {
case c.msgChan <- m:   // Try to send
case <-c.done:         // Client shutting down, skip
default:               // Channel full — drop this message for this client
}
```

### Real-World Analogy
- **Bad:** A mail carrier waits at a full mailbox until the owner empties it (blocks the entire route)
- **Good:** The mail carrier skips the full mailbox and continues delivering to others

### Code Location
`client/client.go:92-104` — `enqueue()`

---

## Pattern 8: Input Continuity (Redraw on Incoming Message)

### The Problem (Plain English)
Alice is typing a message. Midway through, Bob sends a message. If the server just dumps Bob's message onto Alice's terminal, it appears mixed with whatever Alice was typing — the display becomes garbled.

### The Solution (The Pattern)
The server tracks what Alice has typed so far. When an incoming message arrives:
1. Clear Alice's current line (ANSI escape: `\r\033[K`)
2. Print the incoming message
3. Redraw Alice's prompt + whatever she had typed

### The Code

```go
// client/client.go:284-313
func (c *Client) writeWithContinuity(msg string) {
    // 1. Clear the current line
    buf = append(buf, '\r')           // Move cursor to start of line
    buf = append(buf, "\033[K"...)    // ANSI: erase from cursor to end of line

    // 2. Write the incoming message
    buf = append(buf, msg...)

    // 3. Redraw prompt + partial input
    buf = append(buf, c.prompt...)    // e.g., "[2026-02-24 14:30:05][Alice]:"
    buf = append(buf, c.inputBuf...) // e.g., "hell" (what Alice typed so far)

    c.Conn.Write(buf)                // All in one write (no flicker)
}
```

### Real-World Analogy
- **Bad:** Someone shouts in the middle of you writing on a whiteboard, erasing your partial sentence
- **Good:** Someone takes a photo of your partial sentence, writes their message above it, then restores your sentence exactly where you left off

### Code Location
`client/client.go:284-313` — `writeWithContinuity()`

---

## Pattern Summary

| Pattern | Problem | Solution | File |
|---------|---------|----------|------|
| Single Writer | Concurrent writes corrupt output | One goroutine owns the connection | `client.go:232` |
| Signal Channels | Need to tell goroutines "stop" | Close a channel | `client.go:46`, `server.go:40` |
| Lock-and-Copy | Concurrent map access | Copy under lock, work with copy | `server.go:286` |
| Atomic Registration | TOCTOU race | Check + mutate in one lock | `server.go:195` |
| Graceful Shutdown | Clean exit under timeout | Signal → wait → force close | `server.go:105` |
| Atomic File Write | Crash-safe persistence | Write temp → rename | `server.go:554` |
| Non-Blocking Send | Slow client blocks everyone | Drop if channel full | `client.go:92` |
| Input Continuity | Incoming messages garble typing | Clear, write, redraw | `client.go:284` |

---

## What to Read Next

Now see these patterns in action with detailed code walkthroughs: [04 - Line by Line](04-line-by-line.md)
