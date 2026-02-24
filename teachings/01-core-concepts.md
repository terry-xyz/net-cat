# Core Concepts

Before diving into what the code does, you need to understand the language it's written in and the key ideas it relies on.

---

## Reading Go Code: Quick Guide

Go is designed to be simple and readable. Here's the syntax you'll see everywhere:

```go
package server              // Every file belongs to a package (folder name)

import "fmt"                // Bring in other packages (like #include or import)

const MaxClients = 10       // A constant: a value that never changes

type Server struct {        // A "struct" is a bundle of related data
    port    string          //   like a form with fields
    clients map[string]*Client
}

func (s *Server) Start() error {   // A "method" on Server
    // s is "this server"          //   (s *Server) means "attached to Server"
    return nil                     //   error is the return type
}

func add(a, b int) int {   // A standalone function
    return a + b            //   takes two ints, returns one int
}

if err != nil {             // Error check — Go's universal pattern
    return err              //   "if something went wrong, stop and report it"
}

for _, item := range list { // Loop over every item in a list
    fmt.Println(item)       //   _ means "I don't need the index"
}
```

**Key symbols:**
- `:=` means "create a new variable and set its value" (`name := "Alice"`)
- `=` means "change an existing variable" (`name = "Bob"`)
- `*` before a type means "pointer to" (a reference, not a copy)
- `<-` means "send to" or "receive from" a channel

---

## Concept 1: Goroutines (Concurrent Workers)

### What (Simple Definition)
> A goroutine is a lightweight thread — a piece of code that runs simultaneously alongside other code.

### Why (Why This Matters)
A chat server talks to many people at the same time. Without concurrency, the server could only handle one person at a time — everyone else would wait in line.

**Real-world analogy:** A restaurant with one waiter vs. many waiters. One waiter means each table waits while others are served. Many waiters means everyone gets attention at once.

### Where (Files)
- `server/server.go:170` — `go s.handleConnection(conn)` — one goroutine per connection
- `client/client.go:77` — `go c.writeLoop()` — one goroutine per client for writing
- `server/server.go:644` — `go s.startHeartbeat(c)` — one goroutine per client for health checks
- `server/server.go:97` — `go s.startMidnightWatcher()` — one goroutine for log rotation

### How (Code Walkthrough)
```go
// server/server.go:170
// Every time someone connects, we spawn a new goroutine to handle them.
// The "go" keyword is what makes it concurrent — it runs in the background.
go s.handleConnection(conn)
```

Without `go`, the server would be stuck handling one connection and couldn't accept new ones. With `go`, it immediately goes back to waiting for the next connection.

---

## Concept 2: Channels (Communication Pipes)

### What (Simple Definition)
> A channel is a pipe that goroutines use to safely pass data to each other.

### Why (Why This Matters)
When multiple goroutines run at the same time, they can't just share variables freely — that causes data corruption ("race conditions"). Channels give them a safe way to communicate.

**Real-world analogy:** A pneumatic tube in a bank. The teller puts a message in the tube, it travels through, and someone else receives it on the other end. Only one message travels at a time, so nothing gets mixed up.

### Where (Files)
- `client/client.go:45` — `msgChan chan writeMsg` — each client has a channel for outgoing messages
- `client/client.go:46` — `done chan struct{}` — each client has a "shutdown signal" channel
- `server/server.go:28` — `admit chan struct{}` — queue entries have an "you're admitted" signal
- `server/server.go:40` — `quit chan struct{}` — the server has a "shutdown" signal

### How (Code Walkthrough)
```go
// client/client.go:98-103
// Sending a message to a client's channel. Non-blocking: if the channel
// is full, we drop the message rather than freezing the entire server.
select {
case c.msgChan <- m:       // Try to put the message in the pipe
case <-c.done:             // Unless the client is shutting down
default:                   // If the pipe is full, drop it (protect others)
}
```

The `select` statement is like a traffic controller — it picks whichever case is ready first. If both are ready, it picks one randomly.

---

## Concept 3: Mutexes (Locks)

### What (Simple Definition)
> A mutex is a lock that ensures only one goroutine accesses shared data at a time.

### Why (Why This Matters)
The server has data that multiple goroutines need to read and write — the client list, chat history, admin list. Without locks, two goroutines could modify the same data simultaneously and corrupt it.

**Real-world analogy:** A bathroom with a lock. Only one person goes in at a time. Others wait until the lock is released.

### Where (Files)
- `server/server.go:37` — `mu sync.RWMutex` — the server's main lock (read-write)
- `client/client.go:53` — `mu sync.Mutex` — each client has its own lock
- `logger/logger.go:14` — `mu sync.Mutex` — the logger has a lock

### How (Code Walkthrough)
```go
// server/server.go:196-197
// RegisterClient locks the server before checking capacity and adding a client.
// "defer" means "unlock when this function returns, no matter what."
s.mu.Lock()
defer s.mu.Unlock()

// Now only THIS goroutine can read or modify s.clients.
if len(s.clients) >= MaxActiveClients {
    return errCapacityFull
}
```

Go has two types of locks:
- `sync.Mutex` — exclusive lock (one at a time, for reading AND writing)
- `sync.RWMutex` — read-write lock (many readers OR one writer, but not both)

The server uses `RWMutex` because reads (like broadcasting to all clients) happen far more often than writes (like adding/removing a client).

---

## Concept 4: Interfaces and Type Assertions

### What (Simple Definition)
> An interface defines behavior ("what can you do?") rather than identity ("what are you?"). A type assertion checks or converts an interface to a specific type.

### Why (Why This Matters)
Go uses interfaces to write flexible code. For example, `net.Conn` is an interface — it represents "anything that can be read from and written to over a network." The server doesn't care if it's a real TCP connection or a fake one in tests.

**Real-world analogy:** A power outlet is an interface. It doesn't care if you plug in a lamp, a phone charger, or a toaster — anything with a compatible plug works.

### Where (Files)
- `server/handler.go:39` — type assertion: `conn.(*net.TCPConn)` checks if a generic connection is specifically a TCP connection
- `server/server.go:56` — `OperatorOutput io.Writer` — anything that can receive text (stdout, a file, a buffer in tests)

### How (Code Walkthrough)
```go
// server/handler.go:39-42
// conn is a net.Conn (interface). We check: is it actually a TCP connection?
// If yes, enable keepalive. If no (e.g., a pipe in tests), skip it.
if tcpConn, ok := conn.(*net.TCPConn); ok {
    tcpConn.SetKeepAlive(true)             // Only TCP connections have this
    tcpConn.SetKeepAlivePeriod(5 * time.Second)
}
```

The `ok` variable is `true` if the assertion succeeded, `false` otherwise. This pattern is safe — it never crashes, even if the type doesn't match.

---

## Concept 5: Defer (Cleanup Guarantees)

### What (Simple Definition)
> `defer` schedules a function to run when the surrounding function exits — no matter how it exits (return, error, panic).

### Why (Why This Matters)
Resources like files, network connections, and locks must always be cleaned up. `defer` guarantees cleanup even if an error occurs halfway through.

**Real-world analogy:** Writing "close the door" on a sticky note before entering a room. No matter what happens inside the room, you'll see the note and close the door on your way out.

### Where (Files)
- `server/server.go:179` — `defer s.mu.Unlock()` — always release the lock
- `server/handler.go:46` — `defer s.UntrackClient(c)` — always remove from tracking
- `server/handler.go:126-148` — `defer func() { ... }()` — cleanup on disconnect

### How (Code Walkthrough)
```go
// server/handler.go:44-46
// Track this client immediately, and guarantee we untrack it when done.
c := client.NewClient(conn)
s.TrackClient(c)
defer s.UntrackClient(c)   // GUARANTEED to run when handleConnection returns

// ... hundreds of lines of code ...
// Even if there's an error at line 180, UntrackClient WILL run.
```

Multiple `defer` statements run in **reverse order** (last-in, first-out), like a stack of plates.

---

## Concept 6: Maps (Dictionaries)

### What (Simple Definition)
> A map is a lookup table — you give it a key, it gives you a value. Like a phone book: give it a name, get a number.

### Why (Why This Matters)
The server needs to quickly find clients by name, check if IPs are banned, and track admin usernames. Maps provide instant lookup.

### Where (Files)
- `server/server.go:35` — `clients map[string]*client.Client` — name → client
- `server/server.go:48` — `kickedIPs map[string]time.Time` — IP → when the cooldown expires
- `server/server.go:49` — `bannedIPs map[string]bool` — IP → banned?
- `server/server.go:52` — `admins map[string]bool` — username → is admin?

### How (Code Walkthrough)
```go
// server/server.go:201-206
// Check if a name is already taken, then add the client
if _, exists := s.clients[name]; exists {  // Look up by key
    return errors.New("name taken")         // Key exists = name taken
}
s.clients[name] = c                        // Add: name → client
```

```go
// server/server.go:218
// Remove a client by name
delete(s.clients, username)   // Built-in function to remove a map entry
```

---

## Concept 7: Error Handling (Go's Way)

### What (Simple Definition)
> In Go, functions that can fail return an `error` value. The caller checks it immediately.

### Why (Why This Matters)
Go doesn't have exceptions (try/catch). Instead, errors are explicit return values. This forces you to think about what happens when things go wrong — right where they go wrong.

**Real-world analogy:** Instead of an alarm going off somewhere in the building (exception), the person you're talking to says "that didn't work" directly to your face (error return). You deal with it right then and there.

### Where (Files)
This pattern is everywhere. Key examples:
- `main.go:47-50` — checking if the server started
- `server/server.go:84-88` — checking if the listener opened
- `server/handler.go:76-87` — checking if the client sent data during onboarding

### How (Code Walkthrough)
```go
// main.go:47-50
// Start the server. If it fails, print the error and exit.
if err := srv.Start(); err != nil {   // "if starting fails..."
    fmt.Fprintf(os.Stderr, "Error: %v\n", err)
    os.Exit(1)                        // Exit with error code 1
}
```

The pattern `if err != nil { handle it }` is the single most common pattern in Go. You'll see it hundreds of times.

---

## Concept 8: The `select` Statement

### What (Simple Definition)
> `select` waits for one of several channel operations to be ready, then executes that one.

### Why (Why This Matters)
When a goroutine needs to wait for multiple things at once (a message arriving, a timer firing, a shutdown signal), `select` lets it respond to whichever happens first.

**Real-world analogy:** Sitting by your phone, your doorbell, and a timer. You respond to whichever one goes off first.

### Where (Files)
- `server/server.go:163-168` — accept loop checks for shutdown
- `server/server.go:657-707` — heartbeat checks for tick, done, or quit
- `server/server.go:724-730` — midnight watcher checks for timer or shutdown
- `client/client.go:237-277` — write loop checks for messages or done

### How (Code Walkthrough)
```go
// server/server.go:163-168
// The accept loop uses select to check: did we get a connection error
// because the server is shutting down? Or was it a transient error?
select {
case <-s.quit:    // quit channel is closed → server is shutting down
    return        // Stop accepting connections
default:          // Any other error
    continue      // Ignore it, try again
}
```

---

## Summary: How These Concepts Connect

```
main.go starts the server
    │
    ▼
Server.Start() opens a TCP listener
    │
    ▼
acceptLoop() waits for connections (LOOP)
    │
    ├─── New connection arrives
    │    │
    │    ▼
    │    go handleConnection()        ← GOROUTINE: one per client
    │         │
    │         ├─ NewClient()          ← creates CHANNEL (msgChan)
    │         │     └─ go writeLoop() ← GOROUTINE: reads from channel
    │         │
    │         ├─ MUTEX lock/unlock    ← protects shared data
    │         │
    │         ├─ RegisterClient()     ← writes to MAP (clients)
    │         │
    │         ├─ go startHeartbeat()  ← GOROUTINE: health check
    │         │     uses SELECT to wait for tick/done/quit
    │         │
    │         └─ Message loop         ← reads input, dispatches commands
    │              checks ERROR returns on every read
    │              uses DEFER for guaranteed cleanup
    │
    └─── Loop back to wait for next connection
```

---

## What to Read Next

Now that you understand the building blocks, see how data flows through the system: [02 - Data Flow](02-data-flow.md)
