# Lesson 01: Core Concepts

## Reading Go Code: Quick Guide

Before diving into the concepts, here's a cheat sheet for Go syntax you'll see throughout:

```go
package server              // This file belongs to the "server" package

import "fmt"                // Pull in another package (like #include or import)

const MaxClients = 10       // A constant — a value that never changes

type Server struct {        // A struct — a container holding related data together
    port   string           //   field: the port number (text)
    clients map[string]*Client // field: a dictionary mapping names to Client pointers
    mu     sync.RWMutex     //   field: a read-write lock for thread safety
}

func (s *Server) Start() error {  // A method on Server. Returns an error (or nil).
    // s is the receiver — the Server this method was called on
}

func isValidPort(s string) bool { // A standalone function. Takes string, returns true/false.
}

if err != nil {             // "If there was an error..."
    return err              //   "...send the error back to whoever called us"
}

for _, name := range names { // Loop over every item in the "names" slice
}

go s.handleConnection(conn)  // Launch this function in a separate goroutine (background thread)

defer c.Close()               // "When this function returns, run c.Close() automatically"

chan struct{}                  // A channel — a pipe for goroutines to communicate
```

---

## Concept 1: Goroutines (Lightweight Threads)

### What (Simple Definition)
> A goroutine is a function running in the background, like a worker on their own task. Go can run thousands of them simultaneously.

### Why (Why This Matters)
A chat server must handle many users at once. If the server could only talk to one user at a time, everyone else would wait in line. Goroutines let the server have a separate conversation with every connected user **at the same time**.

**Real-world analogy:** A restaurant with one waiter (single-threaded) vs. a restaurant with one waiter per table (goroutines). Each waiter handles their own table independently.

### Where (Files)
- `main.go:35` — signal handler goroutine
- `main.go:45` — operator terminal goroutine
- `server/server.go:165` — `go s.handleConnection(conn)` — one goroutine per TCP connection
- `client/client.go:78` — `go c.writeLoop()` — one write goroutine per client
- `server/server.go:243` — heartbeat probe goroutine
- `server/server.go:292` — midnight watcher goroutine

### How (Code Walkthrough)
```go
// server/server.go:154-167  — The accept loop
func (s *Server) acceptLoop() {
    for {
        conn, err := s.listener.Accept()   // Block until someone connects
        if err != nil {
            select {
            case <-s.quit:                 // Server shutting down? Stop accepting
                return
            default:
                continue                   // Temporary error — try again
            }
        }
        go s.handleConnection(conn)        // Spawn a goroutine for this connection.
        // The loop IMMEDIATELY goes back to waiting for the NEXT connection.
        // It doesn't wait for handleConnection to finish.
    }
}
```

**Key Insight:** The `go` keyword is what makes this concurrent. Without it, the server would handle one user at a time.

---

## Concept 2: Channels (Goroutine Communication Pipes)

### What (Simple Definition)
> A channel is a pipe that lets goroutines send data to each other safely. One goroutine puts data in, another takes it out.

### Why (Why This Matters)
When multiple goroutines need to coordinate, they can't just share variables freely (that causes data races — two workers editing the same paper at once). Channels provide a safe way to pass messages between goroutines without sharing memory.

**Real-world analogy:** A conveyor belt in a factory. The assembly worker puts items on the belt, the packaging worker takes them off. They never touch the same item at the same time.

### Where (Files)
- `client/client.go:46` — `msgChan chan writeMsg` — buffered channel (4096 items) for outgoing messages
- `client/client.go:47` — `done chan struct{}` — signal channel: "this client is shutting down"
- `server/server.go:18` — `admit chan struct{}` — signal channel: "you've been admitted from the queue"
- `server/server.go:31` — `quit chan struct{}` — signal channel: "the server is shutting down"

### How (Code Walkthrough)
```go
// client/client.go:93-105  — Sending a message (non-blocking)
func (c *Client) enqueue(m writeMsg) {
    select {
    case <-c.done:          // Is the client already closed?
        return              //   Don't bother sending
    default:
    }
    select {
    case c.msgChan <- m:    // Try to put the message on the conveyor belt
    case <-c.done:          // Client closed while we were trying?
    default:                // Belt is full (4096 items!) — drop the message
        // This protects the server: a slow client can't block everyone else
    }
}
```

**Key Insight:** The `done` channel uses a pattern called "signal channel." It carries no data — it's just either open or closed. When closed, every `select` case reading from it fires immediately. This is how Go broadcasts "stop" to many goroutines at once.

---

## Concept 3: Mutexes (Locks for Shared Data)

### What (Simple Definition)
> A mutex is a lock on a shared resource. Only one goroutine can hold the lock at a time. Others must wait their turn.

### Why (Why This Matters)
The server has maps (dictionaries) that multiple goroutines read and write — like the `clients` map. Without a lock, two goroutines could modify the map simultaneously, corrupting it. A `RWMutex` allows **many readers OR one writer** — reading doesn't block other readers, but writing blocks everyone.

**Real-world analogy:** A bathroom door lock. Many people can look through the window (read), but only one person can be inside (write), and while someone's inside, you wait.

### Where (Files)
- `server/server.go:27` — `mu sync.RWMutex` — protects ALL server state (clients, rooms, bans, admins)
- `client/client.go:54` — `mu sync.Mutex` — protects per-client fields (muted, admin, echoMode, etc.)
- `logger/logger.go:16` — `mu sync.Mutex` — protects the log file handle

### How (Code Walkthrough)
```go
// server/clients.go:30-45  — Registering a new client
func (s *Server) RegisterClient(c *client.Client, name string) error {
    s.mu.Lock()              // Lock the door: "Nobody else touch the client map!"
    defer s.mu.Unlock()      // When this function returns, unlock automatically

    // TOCTOU safety: check + insert happen as one atomic operation
    if _, exists := s.clients[name]; exists {
        return errors.New("name taken")    // Name already in use
    }
    if s.reservedNames[name] {
        return errors.New("name reserved") // "Server" is reserved
    }
    // Name is free — claim it NOW, under the same lock
    c.Username = name
    c.JoinTime = time.Now()
    s.clients[name] = c
    return nil
}
```

**Key Insight:** The check "is this name taken?" and the insert "ok, take it" must happen under the *same* lock. If you unlock between them, another goroutine could steal the name in between. This is called a TOCTOU (Time-of-Check-Time-of-Use) race.

---

## Concept 4: The `defer` Pattern (Cleanup Guarantees)

### What (Simple Definition)
> `defer` schedules a function call to run when the current function returns — no matter how it returns (success, error, or panic).

### Why (Why This Matters)
In a chat server, you MUST clean up when a user disconnects: remove them from the client map, broadcast a leave message, close their connection, and potentially admit someone from the queue. If you forget any of these steps, you get "ghost" users or stuck queues. `defer` guarantees cleanup happens even if the function exits unexpectedly.

**Real-world analogy:** A hotel checkout. No matter how you leave (check out normally, get kicked out, fire alarm), someone needs to clean the room, update the guest list, and give the keycard to the next guest.

### Where (Files)
- `server/handler.go:49` — `defer s.UntrackClient(c)` — always untrack the connection
- `server/handler.go:134-158` — the big cleanup `defer` block after room join
- `client/client.go:120-125` — `defer` via `closeOnce.Do` for idempotent close

### How (Code Walkthrough)
```go
// server/handler.go:134-158  — Cleanup on any exit
defer func() {
    username := c.Username
    currentRoom := c.Room
    switch c.GetDisconnectReason() {
    case "kicked", "banned":
        // Moderation handler already did the cleanup — skip
    default:
        s.RemoveClient(username)       // Remove from client map + room
        reason := c.GetDisconnectReason()
        if reason == "" {
            reason = "voluntary"       // Normal disconnect (user typed /quit or Ctrl+C)
        }
        // Log and broadcast the leave event
        leaveMsg := models.Message{...}
        s.recordRoomEvent(currentRoom, leaveMsg)
        s.BroadcastRoom(currentRoom, models.FormatLeave(username)+"\n", username)
    }
    s.admitFromRoomQueue(currentRoom)  // Let the next queued user in
    s.deleteRoomIfEmpty(currentRoom)   // Clean up empty rooms
    c.Close()                          // Close the TCP connection
}()
```

**Key Insight:** The `defer` checks `c.GetDisconnectReason()` to avoid double-cleanup. If a user was kicked, the kick handler already removed them and broadcast the leave — the defer shouldn't do it again.

---

## Concept 5: The Single-Writer Goroutine Pattern

### What (Simple Definition)
> Only ONE goroutine ever writes to a TCP connection. All other goroutines send messages through a channel, and the single writer drains the channel and writes.

### Why (Why This Matters)
TCP connections are not safe for concurrent writes. If two goroutines write to the same connection simultaneously, the bytes can interleave (half of message A mixed with half of message B). Instead of adding a mutex on every write, this codebase uses a cleaner pattern: one dedicated writer goroutine per connection.

**Real-world analogy:** A secretary. Instead of everyone in the office shouting at the same client on the phone, everyone gives their message to the secretary, who speaks them one at a time.

### Where (Files)
- `client/client.go:236-276` — `writeLoop()` — the single writer goroutine
- `client/client.go:83-84` — `Send()` — how other goroutines queue messages
- `client/client.go:282-311` — `writeWithContinuity()` — the ANSI magic for echo mode

### How (Code Walkthrough)
```go
// client/client.go:236-276  — The writeLoop (simplified)
func (c *Client) writeLoop() {
    for {
        select {
        case msg := <-c.msgChan:     // Wait for a message on the channel
            if !echoMode {
                c.Conn.Write([]byte(msg.data))  // Simple write
            } else {
                // Echo mode: clear line, write message, redraw prompt + partial input
                switch msg.msgType {
                case wmMessage:
                    c.writeWithContinuity(msg.data)
                case wmPrompt:
                    c.prompt = msg.data
                    c.inputBuf = c.inputBuf[:0]  // Clear tracked input
                    c.Conn.Write([]byte(msg.data))
                // ... more cases for echo, backspace, newline
                }
            }
        case <-c.done:               // Client shutting down
            return                   // Stop the writer goroutine
        }
    }
}
```

**Key Insight:** The `writeLoop` is the ONLY function that calls `c.Conn.Write` with data (except for the heartbeat's single null byte). This eliminates an entire class of concurrency bugs.

---

## Concept 6: Message Types and the Type Enum Pattern

### What (Simple Definition)
> Every event in the chat (message, join, leave, kick, etc.) is represented as a `Message` struct with a `Type` field that says what kind of event it is.

### Why (Why This Matters)
A chat server has many types of events. Instead of creating a different struct for each one, this codebase uses a single `Message` struct where the `Type` field determines the meaning of the other fields. This makes logging, history, and display consistent across all event types.

**Real-world analogy:** A hospital form. The same form is used for admissions, discharges, and transfers — but a "Form Type" checkbox at the top tells you which fields to fill in.

### Where (Files)
- `models/message.go:10-20` — the 7 message types
- `models/message.go:32-39` — the `Message` struct and field semantics
- `models/message.go:85-104` — `Display()` — renders the message for a client
- `models/message.go:107-136` — `FormatLogLine()` — renders for the log file
- `models/message.go:139-242` — `ParseLogLine()` — reconstructs from a log line

### How (Code Walkthrough)
```go
// models/message.go:10-20  — The 7 event types
const (
    MsgChat         MessageType = iota  // 0: Regular chat message
    MsgJoin                             // 1: User joined
    MsgLeave                            // 2: User left
    MsgNameChange                       // 3: User changed their name
    MsgAnnouncement                     // 4: Admin announcement
    MsgModeration                       // 5: Kick, ban, mute, etc.
    MsgServerEvent                      // 6: Server start/stop (internal)
)

// The same struct holds ALL event types:
type Message struct {
    Timestamp time.Time
    Sender    string       // Username (or target for moderation)
    Content   string       // Chat text, or action verb ("kicked")
    Type      MessageType  // Which of the 7 types this is
    Extra     string       // Overloaded: old name, reason, admin name
    Room      string       // Which room this event belongs to
}
```

**Key Insight:** The `Extra` field is "overloaded" — its meaning changes based on the `Type`. For `MsgLeave` it's the reason ("voluntary", "kicked"). For `MsgNameChange` it's the old name. For `MsgModeration` it's the admin who performed the action. This is documented in the comment above the struct.

---

## Concept 7: The Privilege System

### What (Simple Definition)
> Every command has a minimum privilege level required to use it. Users have no special privileges, admins can moderate, and the server operator can do everything.

### Why (Why This Matters)
You don't want regular users kicking each other. The privilege system ensures that moderation commands (kick, ban, mute) are only available to admins and the operator, while basic commands (list, help, whisper) are available to everyone.

**Real-world analogy:** A building with keycards. Everyone can enter the lobby (User), managers can enter restricted floors (Admin), and the building owner can access everything (Operator).

### Where (Files)
- `cmd/commands.go:8-12` — privilege level enum
- `cmd/commands.go:23-39` — command registry with `MinPriv` per command
- `cmd/commands.go:66-74` — `GetPrivilegeLevel()` maps booleans to a level
- `server/commands.go:46-92` — `dispatchCommand()` checks privilege before routing

### How (Code Walkthrough)
```go
// cmd/commands.go:8-12  — Three levels, ordered by power
const (
    PrivUser     PrivilegeLevel = iota  // 0: any connected client
    PrivAdmin                           // 1: promoted admin
    PrivOperator                        // 2: server terminal only
)

// server/commands.go:46-58  — Privilege check before execution
func (s *Server) dispatchCommand(c *client.Client, cmdName, args string) bool {
    def, exists := cmd.Commands[cmdName]
    if !exists {
        c.Send("Unknown command: /" + cmdName + ".\n")
        return false
    }
    clientPriv := cmd.GetPrivilegeLevel(c.IsAdmin(), false) // false = not operator
    if def.MinPriv > clientPriv {
        c.Send("Insufficient privileges.\n")   // You don't have the keycard!
        return false
    }
    // ... dispatch to the right handler
}
```

**Key Insight:** The operator terminal (`server/operator.go`) doesn't go through `dispatchCommand`. It has its own `OperatorDispatch` that skips privilege checks entirely — the operator is always trusted. Certain commands (like `/quit`, `/name`) are explicitly marked as "not applicable" for the operator since they don't have a TCP connection.

---

## What's Next

In the next lesson, you'll trace the complete **data flow** — following a chat message from the moment a user types it to the moment every other user sees it on their screen.
