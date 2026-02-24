# Line-by-Line Walkthroughs

This chapter takes the most important functions in the codebase and explains them **line by line**. Before each walkthrough, there's a plain English summary so you know *what* the code does before you see *how*.

---

## Reading Go Code: Quick Syntax Guide

```go
func name(param Type) ReturnType {  // Function definition
    // param is input, ReturnType is output
}

if condition {     // If statement — no parentheses needed in Go
    // do this if true
}

for i := 0; i < 10; i++ {   // Loop 0 through 9
    // i++ means "add 1 to i"
}

switch value {     // Multi-way branch (like if/else-if chain)
case "a":          // If value == "a"
case "b":          // If value == "b"
default:           // If nothing matched
}

select {           // Like switch, but for channels
case msg := <-ch:  // If channel ch has data, read it
case <-done:       // If done channel is closed
default:           // If nothing is ready right now
}
```

**Symbols refresher:**
- `:=` — create new variable + assign
- `<-` — send to / receive from channel
- `*` — pointer (reference to the original, not a copy)
- `defer` — "run this when the function exits"
- `go` — "run this in the background"

---

## Section 1: The Entry Point — `main()`

**Big Picture:** This is where everything begins. When you run the program, Go calls `main()`. It reads the port number from command-line arguments, creates the server and logger, sets up Ctrl+C handling, and starts accepting connections. Think of it as the "power on" sequence.

**File:** `main.go:11-51`

```go
// INPUT:  command-line arguments (e.g. ./TCPChat 4000)
// OUTPUT: a running server, or an error message and exit

func main() {
    // STEP 1: Default port, overridable via command-line argument
    port := "8989"                  // Default: listen on port 8989
    if len(os.Args) > 2 {          // Too many arguments? (program name + port = 2)
        fmt.Println("[USAGE]: ./TCPChat $port")
        os.Exit(1)                  // Exit with error code 1 (convention: non-zero = failure)
    }
    if len(os.Args) == 2 {         // Exactly one argument provided
        port = os.Args[1]          // Use it as the port
    }
    if !IsValidPort(port) {        // Is it a valid number between 1-65535?
        fmt.Println("[USAGE]: ./TCPChat $port")
        os.Exit(1)
    }

    // STEP 2: Create the server (does NOT start it yet — just allocates memory)
    srv := server.New(port)

    // STEP 3: Create the logger (writes to logs/ folder)
    l, err := logger.New("logs")   // "logs" is the directory name
    if err != nil {                 // If we can't create the logger...
        fmt.Fprintf(os.Stderr, "Warning: could not initialize logger: %v\n", err)
        // Note: we DON'T exit. The server can still run without logging.
    }
    srv.Logger = l                  // Attach logger to server

    // STEP 4: Handle Ctrl+C gracefully
    sigChan := make(chan os.Signal, 1)   // Create a channel for OS signals
    signal.Notify(sigChan, os.Interrupt) // "Send Ctrl+C events to this channel"
    go func() {                          // Background goroutine waiting for Ctrl+C
        <-sigChan                        // Block until Ctrl+C arrives
        srv.Shutdown()                   // Gracefully shut down the server
        for range sigChan {              // Drain any extra Ctrl+C presses
        }                                // (prevents the default "kill process" behavior)
    }()

    // STEP 5: Start the operator terminal (reads commands from keyboard)
    go srv.StartOperator(os.Stdin)  // Runs in background so it doesn't block

    // STEP 6: Start the server (BLOCKS here until shutdown)
    if err := srv.Start(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

**Key Insight:** Notice the `go` keyword on the Ctrl+C handler and operator terminal. Without `go`, the program would get stuck waiting for Ctrl+C and never reach `srv.Start()`. The `go` keyword makes them run concurrently.

---

## Section 2: Port Validation Without Libraries — `IsValidPort()`

**Big Picture:** This function checks if a string is a valid port number (1-65535) using raw byte math instead of the standard library's `strconv.Atoi()`. This is a deliberate choice — the project has zero external dependencies, not even the standard library's string-to-integer converter.

**File:** `main.go:54-69`

```go
// INPUT:  a string like "8989" or "abc" or "99999"
// OUTPUT: true if valid port (1-65535), false otherwise

func IsValidPort(s string) bool {
    if len(s) == 0 {               // Empty string is not a valid port
        return false
    }
    port := 0                       // Accumulator: we'll build the number digit by digit
    for _, b := range []byte(s) {   // Loop over each byte in the string
        if b < '0' || b > '9' {    // Is this byte NOT a digit? (ASCII: '0'=48, '9'=57)
            return false            // Non-digit found — invalid
        }
        port = port*10 + int(b-'0')// Build number: shift left + add new digit
                                    // Example: "89" → 0*10+8=8, then 8*10+9=89
        if port > 65535 {           // Overflow check INSIDE the loop (catches "999999" early)
            return false
        }
    }
    return port >= 1                // Port 0 is reserved — minimum is 1
}
```

**Understanding the math: `port = port*10 + int(b-'0')`**
```
String: "8989"

Step 1: b='8'  →  port = 0*10 + (56-48) = 8
Step 2: b='9'  →  port = 8*10 + (57-48) = 89
Step 3: b='8'  →  port = 89*10 + (56-48) = 898
Step 4: b='9'  →  port = 898*10 + (57-48) = 8989
```

**Key Insight:** `b - '0'` converts an ASCII digit byte to its numeric value. In ASCII, `'0'` is 48, `'1'` is 49, etc. So `'8' - '0'` = `56 - 48` = `8`.

---

## Section 3: Server Creation — `New()`

**Big Picture:** This function creates a server with all its data structures initialized but doesn't start it. It's like building a factory — you set up all the rooms and conveyor belts, but the machines aren't running yet.

**File:** `server/server.go:64-80`

```go
// INPUT:  a port string like "8989"
// OUTPUT: a fully initialized Server, ready to call Start()

func New(port string) *Server {
    return &Server{
        port:       port,                               // Which port to listen on
        clients:    make(map[string]*client.Client),     // name → client lookup (empty)
        allClients: make(map[*client.Client]struct{}),   // ALL connections (for shutdown)
        reservedNames: map[string]bool{
            "Server": true,                              // Nobody can use the name "Server"
        },
        quit:           make(chan struct{}),              // Shutdown signal (not yet closed)
        shutdownDone:   make(chan struct{}),              // "Shutdown finished" signal
        kickedIPs:      make(map[string]time.Time),      // Temporarily blocked IPs (empty)
        bannedIPs:      make(map[string]bool),            // Permanently blocked IPs (empty)
        admins:         make(map[string]bool),            // Known admin usernames (empty)
        adminsFile:     "admins.json",                   // Where admin list is saved
        OperatorOutput: os.Stdout,                       // Operator sees output on terminal
    }
}
```

**Key Insight:** Two client maps exist: `clients` (only fully registered users) and `allClients` (everyone, including those still typing their name or in the queue). This matters during shutdown — you want to notify ALL connections, not just registered ones.

---

## Section 4: Starting the Server — `Start()`

**Big Picture:** This opens a TCP port, loads saved state (admins and chat history), starts the midnight log-rotation watcher, and enters the accept loop. It blocks here forever (until shutdown). Think of it as "turning the lights on and opening the doors."

**File:** `server/server.go:83-101`

```go
// INPUT:  nothing (uses s.port from New())
// OUTPUT: nil on clean shutdown, or an error if the port can't be opened

func (s *Server) Start() error {
    var err error
    s.listener, err = net.Listen("tcp", ":"+s.port)  // Open TCP port
    if err != nil {                                    // Port in use? Permission denied?
        return err                                     // Bubble the error up to main()
    }
    fmt.Printf("Listening on the port :%s\n", s.port) // Console confirmation

    // Log the startup event to the daily log file
    s.Logger.Log(models.Message{
        Timestamp: time.Now(),
        Type:      models.MsgServerEvent,
        Content:   "Server started on port " + s.port,
    })

    s.LoadAdmins()                // Read admins.json → populate s.admins map
    s.RecoverHistory()            // Read today's log → rebuild in-memory history
    go s.startMidnightWatcher()   // Background: clear history at midnight

    s.acceptLoop()                // BLOCKS HERE: loop accepting connections forever
    <-s.shutdownDone              // Wait until Shutdown() finishes all cleanup
    return nil                    // Clean exit
}
```

**Key Insight:** `s.acceptLoop()` is NOT prefixed with `go` — it runs in the current goroutine and blocks. This means `Start()` doesn't return until the server shuts down. The line `<-s.shutdownDone` ensures we don't return to `main()` until `Shutdown()` has finished flushing logs.

---

## Section 5: The Accept Loop — `acceptLoop()`

**Big Picture:** This is the server's main loop. It sits and waits for someone to connect. When they do, it hands them off to a new goroutine and immediately goes back to waiting. It's like a receptionist at the front desk.

**File:** `server/server.go:159-172`

```go
// INPUT:  nothing (uses s.listener)
// OUTPUT: nothing (loops until shutdown)

func (s *Server) acceptLoop() {
    for {                                     // Infinite loop
        conn, err := s.listener.Accept()      // BLOCK: wait for next connection
        if err != nil {                       // Error accepting?
            select {
            case <-s.quit:                    // Is the error because we're shutting down?
                return                        // Yes → exit the loop cleanly
            default:                          // No → some transient network error
                continue                      // Ignore it, try again
            }
        }
        go s.handleConnection(conn)           // Spawn a goroutine for this connection
    }
}
```

**Key Insight:** When `Shutdown()` closes the listener, `Accept()` returns an error. The `select` on `s.quit` distinguishes "we closed on purpose" from "something went wrong." Without this check, the server would log false errors during shutdown.

---

## Section 6: Handling a Connection — `handleConnection()`

**Big Picture:** This is the lifecycle of a single client, from TCP connect to disconnect. It's the longest and most important function. Each client gets its own goroutine running this function. The flow is: IP check → queue check → welcome → name → history → message loop → cleanup.

**File:** `server/handler.go:37-199`

```go
// INPUT:  a raw TCP connection
// OUTPUT: nothing (runs until the client disconnects or is kicked)

func (s *Server) handleConnection(conn net.Conn) {
    // STEP 1: Enable TCP keepalive (OS-level dead peer detection)
    if tcpConn, ok := conn.(*net.TCPConn); ok {  // Type assertion: is this real TCP?
        tcpConn.SetKeepAlive(true)                 // Tell the OS to check if peer is alive
        tcpConn.SetKeepAlivePeriod(5 * time.Second)
    }
    // In tests, conn might be a pipe — not TCP. The "ok" check makes this safe.

    // STEP 2: Create client wrapper and register for shutdown notification
    c := client.NewClient(conn)     // Wraps conn, starts writeLoop goroutine
    s.TrackClient(c)                // Add to allClients (for shutdown broadcast)
    defer s.UntrackClient(c)        // GUARANTEED: remove from allClients on exit

    // STEP 3: Reject blocked IPs immediately
    if blocked, reason := s.IsIPBlocked(c.IP); blocked {
        conn.Write([]byte(reason))  // Direct write (bypass channel — simpler for rejection)
        c.Close()
        return
    }

    // STEP 4: Reject during shutdown
    if s.IsShuttingDown() {
        conn.Write([]byte("Server is shutting down. Goodbye!\n"))
        c.Close()
        return
    }

    // STEP 5: Capacity check — if full, offer queue position
    if !s.checkOrQueue(c) {         // Returns false if client declined queue or disconnected
        c.Close()
        return
    }

    // STEP 6: Welcome banner + name prompt
    c.Send(WelcomeBanner + NamePrompt)  // The penguin ASCII art + "[ENTER YOUR NAME]:"

    // STEP 7: Name validation loop — keeps asking until valid name given
    registered := false
    for {
        name, err := c.ReadLine()   // Read a full line (blocking, line-buffered)
        if err != nil {             // Client disconnected during name entry
            if err != io.EOF {      // EOF is normal (client closed connection)
                s.Logger.Log(...)   // Log unexpected errors
            }
            c.Close()
            return
        }

        if valErr := ValidateName(name); valErr != nil {
            c.Send(valErr.Error() + "\n" + NamePrompt)  // "Name cannot be empty." + retry
            continue
        }
        if s.IsReservedName(name) {
            c.Send("Name '" + name + "' is reserved.\n" + NamePrompt)
            continue
        }
        if err := s.RegisterClient(c, name); err != nil {
            if err == errCapacityFull {       // Race: server filled while typing name
                c.Send("Chat became full. Entering queue...\n")
                if !s.checkOrQueue(c) {       // Re-queue
                    c.Close()
                    return
                }
                c.Send(NamePrompt)            // After re-admission, ask name again
                continue
            }
            c.Send("Name is already taken.\n" + NamePrompt)
            continue
        }
        registered = true
        break                                 // Name accepted!
    }
    if !registered {
        c.Close()
        return
    }

    // STEP 8: Restore admin privileges if this name is in admins.json
    if s.IsKnownAdmin(c.Username) {
        c.SetAdmin(true)
        c.Send("Welcome back, admin!\n")
    }

    // STEP 9: Cleanup deferred — runs when this function exits for ANY reason
    defer func() {
        username := c.Username
        switch c.GetDisconnectReason() {
        case "kicked", "banned":
            // Already handled by the /kick or /ban command — don't double-broadcast
        default:
            s.RemoveClient(username)                // Remove from client map
            reason := c.GetDisconnectReason()
            if reason == "" {
                reason = "voluntary"                // /quit or clean disconnect
            }
            leaveMsg := models.Message{
                Timestamp: time.Now(),
                Sender:    username,
                Type:      models.MsgLeave,
                Extra:     reason,
            }
            s.recordEvent(leaveMsg)                 // Log + history
            s.Broadcast(models.FormatLeave(username)+"\n", username)
        }
        s.admitFromQueue()                          // Let next queued person in
        c.Close()                                   // Close the TCP connection
    }()

    // STEP 10: Send chat history so the new client can catch up
    for _, msg := range s.GetHistory() {
        c.Send(msg.Display() + "\n")
    }

    // STEP 11: Switch to interactive mode (character-at-a-time echo)
    c.SetEchoMode(true)
    c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))

    // STEP 12: Broadcast join + start heartbeat
    joinMsg := models.Message{
        Timestamp: time.Now(),
        Sender:    c.Username,
        Type:      models.MsgJoin,
    }
    s.recordEvent(joinMsg)
    s.Broadcast(models.FormatJoin(c.Username)+"\n", c.Username)
    c.SetLastInput(time.Now())
    go s.startHeartbeat(c)

    // STEP 13: The main message loop — runs forever until disconnect
    for {
        line, err := c.ReadLineInteractive()  // Read char-by-char with echo
        if err != nil {
            c.SetDisconnectReason("drop")     // Connection lost unexpectedly
            if err != io.EOF {
                s.Logger.Log(...)             // Log the error
            }
            return                             // Triggers deferred cleanup above
        }
        c.SetLastInput(time.Now())             // Heartbeat: "I'm alive"
        cmdName, args, isCmd := cmd.ParseCommand(line)
        if isCmd {
            if s.dispatchCommand(c, cmdName, args) {
                return                         // /quit → triggers deferred cleanup
            }
            continue
        }
        s.handleChatMessage(c, line)           // Normal chat message
    }
}
```

**Key Insight:** The `defer func()` block at step 9 is the safety net. No matter *how* the client disconnects — voluntary /quit, kicked, banned, network drop, server shutdown — this cleanup code runs. The `switch c.GetDisconnectReason()` prevents double-broadcasting when moderation commands have already handled the announcement.

---

## Section 7: Creating a Client — `NewClient()`

**Big Picture:** This wraps a raw TCP connection into a Client struct with its own write channel and background writer goroutine. The moment you call `NewClient()`, a goroutine starts running that will handle all writes to this connection.

**File:** `client/client.go:67-79`

```go
// INPUT:  a raw network connection (net.Conn)
// OUTPUT: a fully initialized Client with its writeLoop already running

func NewClient(conn net.Conn) *Client {
    c := &Client{
        Conn:    conn,                              // The TCP connection
        IP:      conn.RemoteAddr().String(),         // e.g. "192.168.1.5:54321"
        msgChan: make(chan writeMsg, msgChanSize),   // Buffered channel (4096 slots)
        done:    make(chan struct{}),                 // Shutdown signal (not yet closed)
    }
    scanner := bufio.NewScanner(conn)                // Line-based reader
    scanner.Buffer(make([]byte, 4096), maxLineLength) // Up to 1MB lines
    c.scanner = scanner
    go c.writeLoop()                                  // Start the writer goroutine NOW
    return c
}
```

**Key Insight:** The channel size of 4096 is important. During a burst (e.g., replaying history to a new client), hundreds of messages might be queued. If the channel were too small, messages would be dropped (the `enqueue` function drops on full). 4096 is large enough for most scenarios.

---

## Section 8: The Non-Blocking Send — `enqueue()`

**Big Picture:** This is how any goroutine sends a message to a client's write channel. It uses a two-step select to be completely non-blocking: if the client is closing or the channel is full, we skip rather than freeze.

**File:** `client/client.go:92-104`

```go
// INPUT:  a writeMsg to send
// OUTPUT: nothing (fire-and-forget)

func (c *Client) enqueue(m writeMsg) {
    // STEP 1: Quick check — is this client already shutting down?
    select {
    case <-c.done:     // done channel is closed → client is gone
        return         // Don't bother trying to send
    default:           // Client is alive, proceed
    }

    // STEP 2: Try to put the message in the channel
    select {
    case c.msgChan <- m:   // Success! Message queued for writeLoop
    case <-c.done:         // Client shut down between step 1 and step 2
    default:               // Channel is full — DROP this message
        // This protects the entire server: one slow client
        // doesn't block broadcasts to all other clients.
    }
}
```

**Why two selects instead of one?** The first `select` is a fast-path optimization. If the client is already closed, we skip the more expensive channel send attempt. The second `select` handles the case where the client closes between the two checks (a narrow race window) and also handles the "channel full" case.

---

## Section 9: The Write Loop — `writeLoop()`

**Big Picture:** This goroutine is the ONLY one that writes to the TCP connection (except heartbeat probes). It reads messages from the channel and writes them to the connection, handling different message types differently based on whether echo mode is active.

**File:** `client/client.go:235-278`

```go
// INPUT:  reads from c.msgChan
// OUTPUT: writes to c.Conn

func (c *Client) writeLoop() {
    for {                                   // Infinite loop
        select {
        case msg := <-c.msgChan:            // A message arrived in the channel
            c.mu.Lock()
            echo := c.echoMode             // Are we in interactive mode?
            c.mu.Unlock()

            if !echo {
                // PRE-ONBOARDING: raw write (welcome banner, name prompt)
                if _, err := c.Conn.Write([]byte(msg.data)); err != nil {
                    return                  // Write failed → connection is dead
                }
                continue
            }

            // INTERACTIVE MODE: handle each message type differently
            switch msg.msgType {
            case wmMessage:                 // Incoming chat/notification
                c.writeWithContinuity(msg.data)  // Clear line, write msg, redraw input

            case wmPrompt:                  // New prompt after sending a message
                c.prompt = msg.data         // Update tracked prompt
                c.inputBuf = c.inputBuf[:0] // Clear tracked input (user just pressed Enter)
                c.Conn.Write([]byte(msg.data))

            case wmEcho:                    // User typed a printable character
                if len(msg.data) > 0 && len(c.inputBuf) < maxInteractiveBuf {
                    c.inputBuf = append(c.inputBuf, msg.data[0])  // Track what they typed
                }
                c.Conn.Write([]byte(msg.data))  // Echo it back to their terminal

            case wmBackspace:               // User pressed backspace
                if len(c.inputBuf) > 0 {
                    c.inputBuf = c.inputBuf[:len(c.inputBuf)-1]  // Remove last tracked char
                }
                c.Conn.Write([]byte("\b \b"))   // Terminal: move back, space, move back

            case wmNewline:                 // User pressed Enter
                c.inputBuf = c.inputBuf[:0] // Clear tracked input
                c.prompt = ""               // Clear prompt (handler will send new one)
                c.Conn.Write([]byte("\r\n"))
            }

        case <-c.done:                      // Client is shutting down
            return                           // Exit the goroutine
        }
    }
}
```

**Key Insight:** The `inputBuf` and `prompt` fields are ONLY accessed by this goroutine (the writeLoop). This is safe because only one goroutine runs this function per client. No mutex needed for these fields — the single-writer pattern eliminates the need.

---

## Section 10: Input Continuity — `writeWithContinuity()`

**Big Picture:** When Alice is typing and Bob's message arrives, we need to: (1) erase Alice's partially-typed text, (2) show Bob's message, (3) put Alice's text back. All in one write to avoid flicker.

**File:** `client/client.go:284-313`

```go
// INPUT:  msg = the incoming message to display (e.g., "[14:30:05][Bob]:hi\n")
// OUTPUT: writes to c.Conn — clears line, writes msg, redraws prompt + input

func (c *Client) writeWithContinuity(msg string) {
    hasPrompt := len(c.prompt) > 0     // Is there a prompt to redraw?
    hasInput := len(c.inputBuf) > 0    // Has the user typed anything?

    // Pre-calculate total size for a SINGLE allocation (performance)
    size := len(msg)
    if hasPrompt || hasInput {
        size += 4   // "\r\033[K" = 4 bytes (carriage return + ANSI erase)
    }
    if hasPrompt {
        size += len(c.prompt)
    }
    if hasInput {
        size += len(c.inputBuf)
    }

    // Build the entire output in one buffer
    buf := make([]byte, 0, size)
    if hasPrompt || hasInput {
        buf = append(buf, '\r')          // Move cursor to start of line
        buf = append(buf, "\033[K"...)   // ANSI escape: erase from cursor to end of line
    }
    buf = append(buf, msg...)            // The incoming message
    if hasPrompt {
        buf = append(buf, c.prompt...)   // Redraw prompt: "[2026-02-24 14:30:05][Alice]:"
    }
    if hasInput {
        buf = append(buf, c.inputBuf...)  // Redraw what Alice was typing: "hel"
    }

    c.Conn.Write(buf)                     // One single write — no flicker
}
```

**Visual example:**
```
BEFORE (Alice's terminal, she typed "hel"):
[2026-02-24 14:30:05][Alice]:hel█

STEP 1: \r\033[K clears the line:
█                                    (cursor at start, line erased)

STEP 2: Write Bob's message:
[2026-02-24 14:30:10][Bob]:hi
█                                    (cursor on new line)

STEP 3: Redraw prompt + partial input:
[2026-02-24 14:30:10][Bob]:hi
[2026-02-24 14:30:05][Alice]:hel█    (Alice's typing is restored!)
```

---

## Section 11: Reading Input Interactively — `ReadLineInteractive()`

**Big Picture:** Instead of waiting for the user to press Enter (line-buffered), this reads one byte at a time. Each keystroke is immediately sent back (echoed) through the write channel so the writeLoop handles it. This enables the input continuity feature — we know exactly what the user has typed so far.

**File:** `client/client.go:328-376`

```go
// INPUT:  reads from c.Conn one byte at a time
// OUTPUT: the complete line when Enter is pressed (without the newline)

func (c *Client) ReadLineInteractive() (string, error) {
    buf := make([]byte, 1)         // 1-byte buffer
    var line []byte                // Accumulates the full line
    for {
        n, err := c.Conn.Read(buf) // BLOCK: wait for one byte
        if err != nil {
            return "", err          // Connection lost
        }
        if n == 0 {
            continue                // Spurious empty read — try again
        }
        b := buf[0]                 // The byte we received

        // Handle \r\n sequences: some terminals send \r\n for Enter.
        // After processing \r (which returns the line), skip the trailing \n.
        if c.skipLF && b == '\n' {
            c.skipLF = false
            continue
        }
        c.skipLF = false

        switch {
        case b == '\r':                     // Enter key (carriage return variant)
            c.skipLF = true                 // Next byte might be \n — skip it
            c.enqueue(writeMsg{msgType: wmNewline})  // Tell writeLoop to write \r\n
            return string(line), nil        // Return the complete line

        case b == '\n':                     // Enter key (newline variant)
            c.enqueue(writeMsg{msgType: wmNewline})
            return string(line), nil

        case b == 0x7F || b == 0x08:        // Backspace or Delete key
            if len(line) > 0 {
                line = line[:len(line)-1]   // Remove last character from buffer
                c.enqueue(writeMsg{msgType: wmBackspace})  // Tell writeLoop to erase it
            }

        case b == 0x00:                     // Null byte — heartbeat probe echo
            continue                        // Silently ignore

        case b >= 0x20 && b <= 0x7E:        // Printable ASCII (space through ~)
            line = append(line, b)          // Add to line buffer
            c.enqueue(writeMsg{data: string([]byte{b}), msgType: wmEcho})  // Echo it

        default:                            // Control characters (Ctrl+C, etc.)
            continue                        // Silently ignore
        }
    }
}
```

**Key Insight:** This function runs in the handler goroutine (one per client). It does NOT write to the connection itself — all visual feedback goes through `c.enqueue()` → channel → writeLoop. This prevents the handler goroutine and the writeLoop goroutine from both writing to the same connection simultaneously.

---

## Section 12: Command Parsing — `ParseCommand()`

**Big Picture:** Takes a raw input string and decides if it's a command (starts with `/`). If so, splits it into the command name and arguments. Simple string manipulation — no regex or parsing libraries.

**File:** `cmd/commands.go:47-60`

```go
// INPUT:  "/kick Bob" or "hello everyone"
// OUTPUT: ("kick", "Bob", true) or ("", "", false)

func ParseCommand(input string) (name string, args string, isCommand bool) {
    if len(input) == 0 || input[0] != '/' {  // Doesn't start with /
        return "", "", false                   // Not a command
    }
    rest := input[1:]                          // Strip the leading /
    if len(rest) == 0 {
        return "", "", true                    // Lone "/" — isCommand but empty name
    }
    idx := strings.IndexByte(rest, ' ')        // Find the first space
    if idx < 0 {
        return rest, "", true                  // No space → entire rest is command name
    }                                          // e.g., "/list" → ("list", "", true)
    return rest[:idx], strings.TrimSpace(rest[idx+1:]), true
    // e.g., "/kick  Bob" → ("kick", "Bob", true)
    // TrimSpace handles extra spaces between command and args
}
```

---

## Section 13: The Heartbeat — `startHeartbeat()`

**Big Picture:** A goroutine that runs for each client, pinging them every 10 seconds with an invisible null byte. If the ping fails or times out, the client is considered dead and disconnected. This catches "ghost" clients whose network dropped without a clean TCP close.

**File:** `server/server.go:644-709`

```go
// INPUT:  a client to monitor
// OUTPUT: nothing (closes the client if dead)

func (s *Server) startHeartbeat(c *client.Client) {
    // Configuration (with defaults)
    interval := s.HeartbeatInterval       // How often to check (default: 10s)
    if interval == 0 { interval = 10 * time.Second }
    timeout := s.HeartbeatTimeout         // How long to wait for probe (default: 5s)
    if timeout == 0 { timeout = 5 * time.Second }

    ticker := time.NewTicker(interval)    // Fire every 10 seconds
    defer ticker.Stop()                   // Clean up ticker on exit

    for {
        select {
        case <-ticker.C:                  // 10 seconds have passed
            if c.IsClosed() { return }    // Client already gone

            // OPTIMIZATION: Skip probe if client recently sent data
            if time.Since(c.GetLastInput()) < interval {
                continue                   // Active within last 10s — no probe needed
            }

            // Send probe in a SEPARATE goroutine (avoids blocking this loop)
            probeResult := make(chan error, 1)
            start := time.Now()
            go func() {
                _, err := c.Conn.Write([]byte{0})  // Write a null byte
                probeResult <- err                    // Report success/failure
            }()

            select {
            case err := <-probeResult:              // Probe completed
                elapsed := time.Since(start)
                if err != nil {                     // Write failed — client is dead
                    c.SetDisconnectReason("drop")
                    c.Close()
                    return
                }
                if elapsed > timeout/2 {            // Slow but alive — warn
                    c.Send("Connection unstable...\n")
                }

            case <-time.After(timeout):              // Probe timed out (5 seconds)
                c.SetDisconnectReason("drop")
                c.Close()                            // Disconnect them
                return

            case <-c.Done():                         // Client disconnected on their own
                return
            case <-s.quit:                           // Server shutting down
                return
            }

        case <-c.Done():    return                   // Client disconnected
        case <-s.quit:      return                   // Server shutting down
        }
    }
}
```

**Why a separate goroutine for the probe?** The `Write` call could block if the TCP buffer is full (the client's network is congested). Writing in a goroutine lets us use `time.After(timeout)` to give up if it takes too long, without interfering with the writeLoop's own writes.

---

## Section 14: Graceful Shutdown — `Shutdown()`

**Big Picture:** This is the "closing time" procedure. Signal everyone, stop accepting, wait for voluntary exits, force-close stragglers, flush logs, signal completion. The `sync.Once` ensures this only runs once even if called from multiple goroutines.

**File:** `server/server.go:105-157`

```go
// INPUT:  nothing
// OUTPUT: nothing (idempotent — safe to call multiple times)

func (s *Server) Shutdown() {
    s.shutdownOnce.Do(func() {           // Run this block EXACTLY once
        close(s.quit)                     // Signal all goroutines: "we're done"

        // Stop accepting new connections
        if s.listener != nil {
            s.listener.Close()            // Makes acceptLoop's Accept() return error
        }

        // Send goodbye to ALL tracked connections
        s.mu.RLock()
        for c := range s.allClients {
            c.Send("Server is shutting down. Goodbye!\n")
        }
        s.mu.RUnlock()

        // Wait for voluntary disconnects (up to 5 seconds)
        timeout := s.ShutdownTimeout
        if timeout == 0 {
            timeout = 5 * time.Second
        }
        deadline := time.Now().Add(timeout)
        for time.Now().Before(deadline) {  // Poll loop
            s.mu.RLock()
            remaining := len(s.allClients)
            s.mu.RUnlock()
            if remaining == 0 {
                break                       // Everyone left voluntarily!
            }
            time.Sleep(50 * time.Millisecond) // Check every 50ms
        }

        // Force-close anyone still connected
        s.mu.RLock()
        for c := range s.allClients {
            c.Close()
        }
        s.mu.RUnlock()

        // Give handler goroutines time to run their deferred cleanup
        time.Sleep(200 * time.Millisecond)

        // Log shutdown and flush
        s.Logger.Log(models.Message{
            Timestamp: time.Now(),
            Type:      models.MsgServerEvent,
            Content:   "Server shutting down",
        })
        s.Logger.Close()

        close(s.shutdownDone)              // Unblock Start() so it can return
    })
}
```

**Key Insight:** `shutdownOnce.Do()` is critical. Ctrl+C could be pressed while a heartbeat goroutine is also detecting a failed write and calling `Shutdown()`. Without `sync.Once`, the shutdown logic could run twice, potentially closing already-closed channels (which panics in Go).

---

## Section 15: Log File Parsing — `ParseLogLine()`

**Big Picture:** This is the inverse of `FormatLogLine()`. Given a line from the log file like `[2026-02-24 14:30:05] CHAT [Alice]:hello`, it reconstructs a `Message` struct. Used during history recovery on server restart.

**File:** `models/message.go:133-225`

```go
// INPUT:  "[2026-02-24 14:30:05] CHAT [Alice]:hello"
// OUTPUT: Message{Timestamp: ..., Type: MsgChat, Sender: "Alice", Content: "hello"}, nil

func ParseLogLine(line string) (Message, error) {
    line = strings.TrimSpace(line)
    if len(line) == 0 {
        return Message{}, fmt.Errorf("empty line")
    }

    // STEP 1: Extract the timestamp from [YYYY-MM-DD HH:MM:SS]
    if line[0] != '[' {
        return Message{}, fmt.Errorf("invalid format: missing opening bracket")
    }
    closeBracket := strings.IndexByte(line, ']')
    if closeBracket < 2 {
        return Message{}, fmt.Errorf("invalid format: malformed timestamp")
    }

    tsStr := line[1:closeBracket]   // e.g., "2026-02-24 14:30:05"
    var year, month, day, hour, min, sec int
    n, err := fmt.Sscanf(tsStr, "%d-%d-%d %d:%d:%d", &year, &month, &day, &hour, &min, &sec)
    if err != nil || n != 6 {
        return Message{}, fmt.Errorf("invalid timestamp: %s", tsStr)
    }
    ts := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.Local)

    // STEP 2: Parse the type keyword after "] "
    rest := line[closeBracket+2:]   // e.g., "CHAT [Alice]:hello"

    // STEP 3: Branch by type and parse fields
    if strings.HasPrefix(rest, "CHAT ") {
        inner := rest[5:]                    // "[Alice]:hello"
        idx := strings.Index(inner, "]:")    // Find "]:" separator
        sender := inner[1:idx]               // "Alice"
        content := inner[idx+2:]             // "hello"
        return Message{Timestamp: ts, Type: MsgChat, Sender: sender, Content: content}, nil
    }

    if strings.HasPrefix(rest, "JOIN ") {
        return Message{Timestamp: ts, Type: MsgJoin, Sender: rest[5:]}, nil
    }

    // ... similar branches for LEAVE, NAMECHANGE, ANNOUNCE, MOD, SERVER ...
}
```

**Key Insight:** This is hand-written string parsing — no regex, no JSON, no external libraries. It's fast and has zero dependencies, but it's also fragile: if `FormatLogLine()` changes format, `ParseLogLine()` must change too. The two functions are a **matched pair**.

---

## Section 16: The Logger — `Log()`

**Big Picture:** Thread-safe, nil-safe logging to daily files. Automatically switches files at midnight (or when the date changes). The nil-safe pattern means you can call `logger.Log()` even if the logger failed to initialize — it just silently does nothing.

**File:** `logger/logger.go:33-54`

```go
// INPUT:  a Message to log
// OUTPUT: nothing (writes to daily file as a side effect)

func (l *Logger) Log(msg models.Message) {
    if l == nil {                  // Nil-safe: if logger was never created
        return                     // Silently do nothing
    }
    l.mu.Lock()                    // Thread-safe: only one goroutine writes at a time
    defer l.mu.Unlock()

    if l.closed {                  // After Shutdown() called Logger.Close()
        return                     // Don't reopen the file
    }

    date := formatDate(msg.Timestamp)  // "2026-02-24"
    if err := l.ensureFile(date); err != nil {  // Open/switch file if needed
        fmt.Fprintf(os.Stderr, "Logger error: %v\n", err)
        return
    }

    line := msg.FormatLogLine() + "\n"  // "[2026-02-24 14:30:05] CHAT [Alice]:hello\n"
    if _, err := l.file.WriteString(line); err != nil {
        fmt.Fprintf(os.Stderr, "Logger write error: %v\n", err)
    }
}
```

**Key Insight:** The `if l.closed` check prevents a subtle bug: after `Shutdown()` calls `Logger.Close()`, late goroutines (still running their cleanup) might try to log. Without this check, `Log()` would reopen the file. The `closed` flag prevents this.

---

## Summary: Reading Order for the Code

If you want to read the source code yourself, here's the recommended order:

| Order | File | Lines | What You Learn |
|-------|------|-------|----------------|
| 1 | `main.go` | 69 | How the program boots up |
| 2 | `cmd/commands.go` | 71 | Command definitions and parsing |
| 3 | `models/message.go` | 225 | Data shapes and formatting |
| 4 | `logger/logger.go` | 113 | Thread-safe file I/O |
| 5 | `client/client.go` | 376 | Client lifecycle and write loop |
| 6 | `server/server.go` | 1111 | Server state, broadcast, moderation, heartbeat |
| 7 | `server/handler.go` | 805 | Connection handling and command implementations |

Start small (cmd, models, logger), then move to the core (client, server). Each file builds on the ones before it.

---

## What to Read Next

Now put your knowledge to the test with hands-on exercises: [05 - Exercises](05-exercises.md)
