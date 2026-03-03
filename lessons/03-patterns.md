# Lesson 03: Design Patterns

## Why Learn Patterns?

A pattern is a **proven solution to a common problem**. Like cooking techniques: you don't reinvent "sauteing" every time you cook vegetables.

Learning patterns helps you:
1. **Read code faster** — "Oh, this is the single-writer pattern"
2. **Write better code** — Use battle-tested solutions
3. **Communicate** — "Let's use the registry pattern for commands"
4. **Understand WHY** — Not just WHAT the code does

---

## Pattern 1: Single-Writer Goroutine

### The Problem (Plain English)
Multiple goroutines need to write messages to a TCP connection. If two write simultaneously, bytes get interleaved — "Hello" + "World" might become "HWeollrlod". Adding a mutex on every write would work but creates contention.

### The Solution (The Pattern)
Funnel ALL writes through a single goroutine that reads from a buffered channel.

### The Code

```go
// WRONG way: multiple goroutines write directly
func (s *Server) broadcastDirect(msg string) {
    for _, c := range s.clients {
        c.Conn.Write([]byte(msg))  // RACE CONDITION! Multiple goroutines call this!
    }
}

// RIGHT way: send to channel, single writer drains it
func (c *Client) Send(msg string) {
    c.enqueue(writeMsg{data: msg, msgType: wmMessage})  // Non-blocking, safe
}

// Only this goroutine calls Conn.Write:
func (c *Client) writeLoop() {
    for {
        select {
        case msg := <-c.msgChan:
            c.Conn.Write([]byte(msg.data))  // Single writer — no race!
        case <-c.done:
            return
        }
    }
}
```

### Real-World Analogy
- **Bad:** Everyone in the office yelling into the same phone at once
- **Good:** Everyone writes a note, the secretary reads them into the phone one at a time

### Code Location
`client/client.go:236-276` (writeLoop), `client/client.go:83-105` (Send + enqueue)

---

## Pattern 2: Signal Channel (Close-to-Broadcast)

### The Problem (Plain English)
When the server shuts down or a client disconnects, you need to notify *all* goroutines associated with that entity. You can't send to each one individually because you don't always know how many there are.

### The Solution (The Pattern)
Create a `chan struct{}` and close it. In Go, reading from a closed channel returns immediately. Every goroutine's `select` that includes this channel will fire at once — it's a broadcast.

### The Code

```go
// WRONG way: trying to send a "stop" message to each goroutine
func (s *Server) Shutdown() {
    for _, g := range s.goroutines {
        g.stopChan <- true  // What if the goroutine isn't listening? Blocks forever!
    }
}

// RIGHT way: close the channel — ALL readers wake up immediately
func (s *Server) Shutdown() {
    close(s.quit)  // Every select case <-s.quit fires immediately
}

// Every goroutine checks this channel:
func (s *Server) startHeartbeat(c *client.Client) {
    for {
        select {
        case <-ticker.C:
            // do heartbeat probe
        case <-c.Done():    // Client closed? Stop.
            return
        case <-s.quit:      // Server shutting down? Stop.
            return
        }
    }
}
```

### Real-World Analogy
- **Bad:** Calling every employee individually to say "go home"
- **Good:** Pulling the fire alarm — everyone hears it at once

### Code Location
- `server/server.go:31` — `quit chan struct{}` (server shutdown signal)
- `client/client.go:47` — `done chan struct{}` (client close signal)
- `server/server.go:18` — `admit chan struct{}` (queue admission signal)

---

## Pattern 3: sync.Once (Idempotent Operations)

### The Problem (Plain English)
`Close()` might be called from multiple places: the handler detects a disconnect, the heartbeat detects a timeout, the server shuts down. If `Close()` runs twice, it panics (double-close on a channel or connection).

### The Solution (The Pattern)
Wrap the close logic in `sync.Once`. No matter how many goroutines call it, the function body executes exactly once.

### The Code

```go
// WRONG way: no protection against double-close
func (c *Client) Close() {
    close(c.done)    // PANIC if called twice!
    c.Conn.Close()   // PANIC if called twice!
}

// RIGHT way: sync.Once guarantees single execution
func (c *Client) Close() {
    c.closeOnce.Do(func() {
        close(c.done)
        c.Conn.Close()
    })
}
```

### Real-World Analogy
- **Bad:** Multiple people trying to lock the same door — key jams
- **Good:** A self-locking door — first person closes it, everyone else's push is a no-op

### Code Location
- `client/client.go:120-125` — `closeOnce sync.Once`
- `server/server.go:101` — `shutdownOnce sync.Once`

---

## Pattern 4: Coarse-Grained Locking

### The Problem (Plain English)
The server has many related maps (clients, rooms, kicked IPs, banned IPs, admins). If each had its own lock, you'd risk deadlocks (goroutine A holds lock 1, waits for lock 2; goroutine B holds lock 2, waits for lock 1).

### The Solution (The Pattern)
Use a single `RWMutex` for all server state. Simpler, deadlock-free, at the cost of some concurrency (all operations serialize through one lock).

### The Code

```go
// WRONG way: per-map locks (deadlock risk)
type Server struct {
    clientsMu sync.Mutex
    clients   map[string]*Client
    roomsMu   sync.Mutex
    rooms     map[string]*Room
}
func (s *Server) MoveRoom(c *Client, newRoom string) {
    s.clientsMu.Lock()   // Lock 1
    s.roomsMu.Lock()     // Lock 2 — if another goroutine holds roomsMu and wants clientsMu: DEADLOCK
    // ...
}

// RIGHT way: one lock for everything
type Server struct {
    mu      sync.RWMutex
    clients map[string]*Client
    rooms   map[string]*Room
}
func (s *Server) MoveRoom(c *Client, newRoom string) {
    s.mu.Lock()           // One lock — can't deadlock with itself
    delete(oldRoom.clients, c.Username)
    newRoom.clients[c.Username] = c
    s.mu.Unlock()
}
```

### Real-World Analogy
- **Bad:** Each room in a house has its own key, and some tasks require entering two rooms at once — you might get stuck
- **Good:** One master key for the whole house — simpler, and you can never get locked out

### Code Location
`server/server.go:27` — the single `mu sync.RWMutex` that protects all server maps

**Trade-off:** This works well for a chat server with moderate traffic. For a system serving millions of users, per-room locking (with careful ordering) would be worth the complexity.

---

## Pattern 5: Registry Pattern (Commands)

### The Problem (Plain English)
The server has 15 commands, each with a name, usage string, description, and minimum privilege level. You need a central place to define them and a way to look them up by name.

### The Solution (The Pattern)
A `map[string]CommandDef` populated at package init time. A separate `[]string` defines display order. Dispatch is a `switch` statement over the map keys.

### The Code

```go
// cmd/commands.go:23-39  — The registry
var Commands = map[string]CommandDef{
    "list":     {Name: "list", MinPriv: PrivUser, Usage: "/list", Description: "List connected clients"},
    "kick":     {Name: "kick", MinPriv: PrivAdmin, Usage: "/kick <name>", Description: "Kick a user"},
    "promote":  {Name: "promote", MinPriv: PrivOperator, Usage: "/promote <name>", Description: "Promote to admin"},
    // ... 12 more commands
}

// cmd/commands.go:42-46  — Display order (separate from the map)
var CommandOrder = []string{"list", "rooms", "switch", ..., "promote", "demote"}
```

### Real-World Analogy
- A restaurant menu: each dish has a name, price (privilege), and description. The waiter looks up your order in the menu, checks if you can afford it, then sends it to the kitchen.

### Code Location
- `cmd/commands.go:23-46` — registry + ordering
- `server/commands.go:46-92` — dispatch switch statement

---

## Pattern 6: Atomic File Write (Write-Rename)

### The Problem (Plain English)
If the server crashes mid-write to `admins.json`, the file could be half-written and corrupt. On next startup, the admin list would be lost.

### The Solution (The Pattern)
Write to a temporary file first, then atomically rename it over the original. `rename` is an atomic operation on most filesystems — either the old file or the new file exists, never a half-written one.

### The Code

```go
// WRONG way: write directly to the target file
func SaveAdmins(path string, data []byte) {
    os.WriteFile(path, data, 0600)  // Crash here = corrupt file!
}

// RIGHT way: write to temp, then rename
func (s *Server) SaveAdmins() {
    data, _ := json.MarshalIndent(names, "", "  ")
    tmpFile := filepath.Join(dir, ".admins.json.tmp")
    os.WriteFile(tmpFile, data, 0600)     // Write to temp file
    os.Rename(tmpFile, path)              // Atomic swap! Crash-safe.
}
```

### Real-World Analogy
- **Bad:** Erasing a whiteboard and rewriting it — if you're interrupted, the board is half-empty
- **Good:** Writing on a NEW whiteboard, then swapping it with the old one in one motion

### Code Location
`server/admins.go:44-83` — `SaveAdmins()` with temp-file + rename

---

## Pattern 7: Defer-Based Cleanup

### The Problem (Plain English)
A connection handler has many exit paths: normal disconnect, read error, kick, ban, `/quit`. Each exit needs the same cleanup: remove from client map, broadcast leave, admit from queue, close connection. Duplicating this cleanup in every exit path is error-prone.

### The Solution (The Pattern)
Use a single `defer` block after the point where the client is fully registered. The defer runs regardless of how the function exits.

### The Code

```go
// WRONG way: cleanup in every exit path
func handler(conn) {
    // ...
    if err := readMsg(); err != nil {
        removeClient()    // Don't forget!
        broadcastLeave()  // Don't forget!
        admitQueue()      // Don't forget!
        conn.Close()      // Don't forget!
        return
    }
    if isKicked {
        removeClient()    // Duplicated!
        broadcastLeave()  // Duplicated!
        admitQueue()      // Duplicated!
        conn.Close()      // Duplicated!
        return
    }
}

// RIGHT way: one defer block handles all cleanup
func (s *Server) handleConnection(conn net.Conn) {
    // ... onboarding, room join ...
    defer func() {
        s.RemoveClient(username)
        s.BroadcastRoom(room, FormatLeave(username))
        s.admitFromRoomQueue(room)
        c.Close()
    }()
    // Every return from here triggers the defer
}
```

### Real-World Analogy
- **Bad:** A hotel where each way of leaving (checkout, eviction, fire) has a separate cleaning crew
- **Good:** One cleaning crew that always runs, no matter how the guest left

### Code Location
`server/handler.go:134-158` — the main cleanup defer

---

## Pattern 8: FIFO Queue with Channel Admission

### The Problem (Plain English)
Rooms have a max capacity of 10. When a room is full, new users should wait in line (first come, first served). When a slot opens, the longest-waiting user should be admitted.

### The Solution (The Pattern)
A `[]*QueueEntry` slice (append for enqueue, slice-off-front for dequeue). Each entry has an `admit chan struct{}` that is closed when the user is admitted. The waiting goroutine blocks on this channel.

### The Code

```go
// The queue entry — each waiting user gets one
type QueueEntry struct {
    client *client.Client
    admit  chan struct{}     // Closed when admitted
}

// Admitting the next user:
func (s *Server) admitFromRoomQueue(roomName string) {
    // ... under lock ...
    entry := r.queue[0]          // First in line
    r.queue = r.queue[1:]        // Remove from queue
    close(entry.admit)           // Signal: "you're in!"
}

// The waiting user's goroutine:
select {
case <-entry.admit:              // Blocks until close(entry.admit)
    // Admitted! Proceed to join the room.
case <-s.quit:
    // Server shutting down — give up.
case <-readDone:
    // User disconnected while waiting — give up.
}
```

### Real-World Analogy
- A doctor's waiting room with numbered tickets. When the doctor is ready, they call the lowest number. The patient hears their number and walks in.

### Code Location
- `server/server.go:16-19` — `QueueEntry` struct
- `server/room.go:211-233` — `admitFromRoomQueue`
- `server/handler.go:295-332` — `waitForRoomAdmission`

---

## Pattern 9: Nil-Safe Methods

### The Problem (Plain English)
The logger is optional — the server can run without one (for testing or when disk is unavailable). Every place that calls `s.Logger.Log(...)` would need a nil check first.

### The Solution (The Pattern)
Make every method on `Logger` check `if l == nil` at entry and return immediately. This means callers never need nil checks — calling `nil.Log(msg)` is safe.

### The Code

```go
// WRONG way: callers must check
func (s *Server) recordEvent(msg Message) {
    if s.Logger != nil {        // Every caller needs this!
        s.Logger.Log(msg)
    }
}

// RIGHT way: methods are nil-safe
func (l *Logger) Log(msg models.Message) {
    if l == nil {               // Nil logger? No-op.
        return
    }
    // ... actual logging ...
}

// Callers are clean:
func (s *Server) recordEvent(msg Message) {
    s.Logger.Log(msg)           // Safe even if Logger is nil
}
```

### Real-World Analogy
- **Bad:** Before every letter, checking if the mailbox exists
- **Good:** A mailbox that, if it doesn't exist, simply doesn't deliver — no crash, no error

### Code Location
`logger/logger.go:33-36`, `logger/logger.go:57-60`, `logger/logger.go:73-76` — nil checks in `Log`, `FilePath`, `Close`

---

## Pattern 10: Manual Insertion Sort (No stdlib)

### The Problem (Plain English)
Several places need sorted lists (room names, client names, admin names). The standard library's `sort` package could be used, but this codebase avoids all external imports.

### The Solution (The Pattern)
Use insertion sort — simple, in-place, and O(n log n) for small n. The same algorithm appears 6+ times across the codebase.

### The Code

```go
// Insertion sort — used everywhere for small slices
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

### Real-World Analogy
- Sorting playing cards in your hand: pick up each card and insert it in the right position among the cards you've already sorted.

### Code Location
- `server/room.go:148-157` — `GetRoomNames`
- `server/room.go:184-193` — `GetRoomClientNames`
- `server/commands.go:116-124` — `cmdList`
- `server/admins.go:58-66` — `SaveAdmins`
- `server/operator.go:109-117` — `operatorCmdList`

---

## What's Next

In the next lesson, you'll go through the code **line by line** for the most important functions.
