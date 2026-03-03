# Lesson 04: Line-by-Line Walkthrough

## Reading Go Code: Quick Syntax Guide

```go
func name(param Type) ReturnType {  // Function: takes param, returns ReturnType
    // param is input, ReturnType is output
}

if condition {     // If: do this block only when condition is true
    // ...
}

for i := 0; i < 10; i++ {   // Loop: i goes 0,1,2,...,9
    // i++ means "add 1 to i"
}

for _, item := range slice { // Loop over every item. _ means "ignore the index"
}

switch value {                // Switch: like multiple if-else
case "a":                     //   if value == "a"
    doA()
case "b":                     //   if value == "b"
    doB()
default:                      //   otherwise
    doDefault()
}

select {                      // Select: wait for the FIRST channel to be ready
case msg := <-ch:             //   got a message from ch
case <-done:                  //   done channel was closed
default:                      //   none are ready — do this instead (non-blocking)
}

:=                            // Declare AND assign (short form)
=                             // Assign to existing variable
<-                            // Read from channel (or send with ch <- val)
```

---

## Section 1: Program Entry Point

**Big Picture:** This is where everything starts. The `main` function parses the port, creates the server and logger, sets up a signal handler for graceful Ctrl+C shutdown, and starts the server.

**File:** `main.go:11-51`

```go
// INPUT:  Command-line arguments (optional port number)
// OUTPUT: A running TCP server (blocks until shutdown)

func main() {
    // STEP 1: Default port is 8989 unless overridden by CLI argument
    port := "8989"
    if len(os.Args) > 2 {                    // Too many arguments?
        fmt.Println("[USAGE]: ./TCPChat $port")
        os.Exit(1)                            // Exit with error code
    }
    if len(os.Args) == 2 {                    // Exactly one argument = custom port
        port = os.Args[1]
    }
    if !isValidPort(port) {                   // Validate: digits only, 1-65535
        fmt.Println("[USAGE]: ./TCPChat $port")
        os.Exit(1)
    }

    // STEP 2: Create the server instance (does NOT start listening yet)
    srv := server.New(port)

    // STEP 3: Create the logger (writes to logs/ directory)
    l, err := logger.New("logs")
    if err != nil {
        fmt.Fprintf(os.Stderr, "Warning: could not initialize logger: %v\n", err)
    }
    srv.Logger = l                            // Attach logger to server

    // STEP 4: Handle Ctrl+C gracefully
    sigChan := make(chan os.Signal, 1)         // Buffered channel for OS signals
    signal.Notify(sigChan, os.Interrupt)      // Register for SIGINT (Ctrl+C)
    go func() {                               // Background goroutine waits for signal
        <-sigChan                             // Block until Ctrl+C
        srv.Shutdown()                        // Graceful shutdown
        for range sigChan {                   // Drain further signals (don't let Go's
        }                                     // default handler kill the process)
    }()

    // STEP 5: Start operator terminal (reads commands from stdin in background)
    go srv.StartOperator(os.Stdin)

    // STEP 6: Start the TCP server (blocks here until shutdown completes)
    if err := srv.Start(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

**Key Insight:** `srv.Start()` blocks until `Shutdown()` is called. The main goroutine sits in `Start()` running the accept loop, while the signal handler and operator terminal run in separate goroutines.

---

## Section 2: Port Validation (No stdlib)

**Big Picture:** Validates that a string is a number between 1 and 65535 without importing `strconv`. This is a deliberate choice to keep the binary minimal with zero non-stdlib imports.

**File:** `main.go:54-69`

```go
// INPUT:  A string like "8989" or "abc"
// OUTPUT: true if it's a valid TCP port (1-65535), false otherwise

func isValidPort(s string) bool {
    if len(s) == 0 {                          // Empty string? Not a port.
        return false
    }
    port := 0
    for _, b := range []byte(s) {             // Check each byte (character)
        if b < '0' || b > '9' {               // Not a digit? Not a port.
            return false
        }
        port = port*10 + int(b-'0')           // Build the number digit by digit
        // '0' is ASCII 48, '9' is ASCII 57
        // b-'0' converts ASCII digit to actual number: '5'-'0' = 5
        if port > 65535 {                     // Already too large? Stop early.
            return false
        }
    }
    return port >= 1                          // Port 0 is not valid
}
```

**Key Insight:** `b - '0'` converts an ASCII digit character to its numeric value. The byte `'5'` has value 53, and `'0'` has value 48. So `53 - 48 = 5`. This is how you parse numbers without a library.

---

## Section 3: Connection Handler Lifecycle

**Big Picture:** This is the heart of the server. Every TCP connection gets its own goroutine running `handleConnection`. It manages the entire lifecycle: IP check, name validation, room selection, queueing, message loop, and cleanup.

**File:** `server/handler.go:40-209`

```go
// INPUT:  A raw TCP connection
// OUTPUT: None (runs until the client disconnects or is kicked)

func (s *Server) handleConnection(conn net.Conn) {
    // STEP 1: Enable TCP keepalive for dead peer detection at the OS level
    if tcpConn, ok := conn.(*net.TCPConn); ok {  // Type assertion: is this a real TCP conn?
        tcpConn.SetKeepAlive(true)                // Ask the OS to probe periodically
        tcpConn.SetKeepAlivePeriod(5 * time.Second)
    }

    // STEP 2: Wrap the raw connection in our Client struct
    c := client.NewClient(conn)                   // Also starts the writeLoop goroutine
    s.TrackClient(c)                              // Add to allClients (for shutdown broadcast)
    defer s.UntrackClient(c)                      // Remove on exit — guaranteed by defer

    // STEP 3: Check IP against kick/ban lists BEFORE sending anything
    if blocked, reason := s.IsIPBlocked(c.IP); blocked {
        conn.Write([]byte(reason))                // Direct write (not via writeLoop)
        c.Close()                                 // because we want guaranteed delivery
        return
    }

    // STEP 4: Reject if server is shutting down
    if s.IsShuttingDown() {
        conn.Write([]byte("Server is shutting down. Goodbye!\n"))
        c.Close()
        return
    }

    // STEP 5: Send welcome banner + name prompt
    c.Send(WelcomeBanner + NamePrompt)

    // STEP 6: Name validation loop — retries until valid + unique
    registered := false
    for {
        name, err := c.ReadLine()                 // Blocking read (line-buffered)
        if err != nil {                           // Connection dropped during onboarding
            c.Close()
            return
        }
        if valErr := validateName(name); valErr != nil {
            c.Send(valErr.Error() + "\n" + NamePrompt)  // "Name too long." + reprompt
            continue
        }
        if s.IsReservedName(name) {
            c.Send("Name '" + name + "' is reserved.\n" + NamePrompt)
            continue
        }
        if err := s.RegisterClient(c, name); err != nil {
            c.Send("Name is already taken.\n" + NamePrompt)
            continue
        }
        registered = true
        break                                     // Success! Name is claimed.
    }

    // STEP 7: Auto-restore admin privileges
    if s.IsKnownAdmin(c.Username) {
        c.SetAdmin(true)
        c.Send("Welcome back, admin!\n")
    }

    // STEP 8: Room selection
    roomName := s.readRoomChoice(c)               // Shows room list, reads choice
    if roomName == "" {                            // Client disconnected
        s.RemoveClient(c.Username)
        c.Close()
        return
    }

    // STEP 9: Queue if room is full
    if !s.checkOrQueueRoom(c, roomName) {          // Returns false if user declines queue
        s.RemoveClient(c.Username)
        c.Close()
        return
    }

    // STEP 10: Join the room
    s.mu.Lock()
    s.JoinRoom(c, roomName)
    s.mu.Unlock()

    // STEP 11: The big cleanup defer — runs on ANY exit from here
    defer func() {
        // ... (covered in Lesson 02, Flow 2)
        // Removes from client map, broadcasts leave, admits from queue, closes conn
    }()

    // STEP 12: Deliver room history to the new user
    for _, msg := range s.GetRoomHistory(c.Room) {
        c.Send(msg.Display() + "\n")
    }

    // STEP 13: Switch to interactive echo mode
    c.SetEchoMode(true)
    c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))

    // STEP 14: Broadcast join + start heartbeat
    joinMsg := models.Message{Timestamp: time.Now(), Sender: c.Username, Type: models.MsgJoin}
    s.recordRoomEvent(c.Room, joinMsg)
    s.BroadcastRoom(c.Room, models.FormatJoin(c.Username)+"\n", c.Username)
    c.SetLastInput(time.Now())
    go s.startHeartbeat(c)                         // Start health monitoring

    // STEP 15: Message loop — runs until disconnect, /quit, or kick
    for {
        line, err := c.ReadLineInteractive()       // Byte-by-byte reading with echo
        if err != nil {
            c.SetDisconnectReason("drop")          // Connection died
            return                                 // Triggers the defer cleanup
        }
        c.SetLastInput(time.Now())                 // Reset heartbeat timer
        cmdName, args, isCmd := cmd.ParseCommand(line)
        if isCmd {
            if s.dispatchCommand(c, cmdName, args) {
                return                             // /quit returns true — exit
            }
            continue
        }
        s.handleChatMessage(c, line)               // Regular chat message
    }
}
```

**Key Insight:** The function has a clear linear flow: IP check → name → room → queue → join → loop. The `defer` at step 11 handles ALL cleanup, and the `GetDisconnectReason()` check prevents double-cleanup when moderation handlers already did the work.

---

## Section 4: Interactive Byte-by-Byte Reading

**Big Picture:** After onboarding, the server reads one byte at a time from the client. This enables real-time character echo and backspace handling, creating a responsive terminal experience.

**File:** `client/client.go:326-374`

```go
// INPUT:  Bytes from the TCP connection, one at a time
// OUTPUT: A complete line (string) when the user presses Enter

func (c *Client) ReadLineInteractive() (string, error) {
    buf := make([]byte, 1)                     // Single-byte buffer
    var line []byte                             // Accumulates the full line
    for {
        n, err := c.Conn.Read(buf)             // Read exactly 1 byte (blocks)
        if err != nil {
            return "", err                     // Connection broken
        }
        if n == 0 {
            continue                           // Shouldn't happen, but safe
        }
        b := buf[0]                            // The byte we just read

        // Handle \r\n line endings: after seeing \r, skip the next \n
        if c.skipLF && b == '\n' {
            c.skipLF = false
            continue
        }
        c.skipLF = false

        switch {
        case b == '\r':                        // Carriage return = Enter (some terminals)
            c.skipLF = true                    // Skip the \n that might follow
            c.enqueue(writeMsg{msgType: wmNewline})
            return string(line), nil           // Return the completed line

        case b == '\n':                        // Newline = Enter (other terminals)
            c.enqueue(writeMsg{msgType: wmNewline})
            return string(line), nil

        case b == 0x7F || b == 0x08:           // Backspace or Delete key
            if len(line) > 0 {
                line = line[:len(line)-1]      // Remove last character
                c.enqueue(writeMsg{msgType: wmBackspace})
            }

        case b == 0x00:                        // Null byte from heartbeat probe
            continue                           // Silently ignore

        case b >= 0x20 && b <= 0x7E:           // Printable ASCII (space to ~)
            line = append(line, b)             // Add to line buffer
            c.enqueue(writeMsg{data: string([]byte{b}), msgType: wmEcho})

        default:                               // Any other control character
            continue                           // Ignore (Ctrl+A, escape sequences, etc.)
        }
    }
}
```

**Key Insight:** The `\r\n` handling is critical. Windows terminals send `\r\n` (two bytes) for Enter. Without the `skipLF` flag, the server would see TWO line endings — one for `\r` and one for `\n` — submitting the same line twice.

---

## Section 5: Command Parsing

**Big Picture:** This pure function splits user input like `/kick alice` into a command name (`"kick"`) and arguments (`"alice"`). It's in a separate package (`cmd`) because it has no dependencies on the server.

**File:** `cmd/commands.go:50-63`

```go
// INPUT:  "/kick alice" or "hello everyone"
// OUTPUT: (name="kick", args="alice", isCommand=true)
//     or  (name="", args="", isCommand=false)

func ParseCommand(input string) (name string, args string, isCommand bool) {
    if len(input) == 0 || input[0] != '/' {    // Doesn't start with /? Not a command.
        return "", "", false
    }
    rest := input[1:]                          // Strip the leading /
    if len(rest) == 0 {
        return "", "", true                    // Just "/" alone — is a command, but empty
    }
    idx := strings.IndexByte(rest, ' ')        // Find the first space
    if idx < 0 {
        return rest, "", true                  // No space: "/list" → name="list", args=""
    }
    return rest[:idx], strings.TrimSpace(rest[idx+1:]), true
    // "/kick  alice  " → name="kick", args="alice"
}
```

**Key Insight:** This function is *pure* — it has no side effects, no I/O, no state. It just transforms a string. This makes it trivially testable and reusable.

---

## Section 6: The Heartbeat Probe

**Big Picture:** Dead clients (crashed terminal, network dropout) don't send a TCP FIN. Without active probing, they'd remain in the client list forever as "ghost" users. The heartbeat writes a null byte (`\x00`) — invisible to terminals — and watches for errors.

**File:** `server/server.go:216-281`

```go
// INPUT:  A connected client
// OUTPUT: None (closes the client if the connection is dead)

func (s *Server) startHeartbeat(c *client.Client) {
    // Use configured intervals or defaults (10s probe, 5s timeout)
    interval := s.HeartbeatInterval
    if interval == 0 { interval = 10 * time.Second }
    timeout := s.HeartbeatTimeout
    if timeout == 0 { timeout = 5 * time.Second }

    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:                              // Time to probe
            if c.IsClosed() { return }                // Already closed? Done.

            // Active sender exemption: if the user just typed something,
            // they're clearly alive — skip the probe
            if time.Since(c.GetLastInput()) < interval {
                continue
            }

            // Write the probe in a separate goroutine to avoid blocking
            probeResult := make(chan error, 1)
            start := time.Now()
            go func() {
                _, err := c.Conn.Write([]byte{0})    // Null byte — invisible to terminals
                probeResult <- err
            }()

            select {
            case err := <-probeResult:                // Probe completed
                if err != nil {                       // Write failed = dead connection
                    c.SetDisconnectReason("drop")
                    c.Close()
                    return
                }
                if time.Since(start) > timeout/2 {    // Slow but alive
                    c.Send("Connection unstable...\n")
                }
            case <-time.After(timeout):               // 5 seconds elapsed, no response
                c.SetDisconnectReason("drop")
                c.Close()                             // Kill the ghost
                return
            case <-c.Done():                          // Client disconnected normally
                return
            case <-s.quit:                            // Server shutting down
                return
            }

        case <-c.Done():
            return
        case <-s.quit:
            return
        }
    }
}
```

**Key Insight:** The probe uses a goroutine (`go func() { Conn.Write... }`) instead of `SetWriteDeadline` because a deadline on the connection would interfere with the writeLoop's normal writes. By probing in a goroutine with a `time.After` timeout, we detect dead clients without touching the connection's deadline state.

---

## Section 7: Log Recovery (ParseLogLine)

**Big Picture:** When the server restarts, it reads today's log file and reconstructs the in-memory chat history. This function is the parser that converts a log line back into a `Message` struct.

**File:** `models/message.go:139-242`

```go
// INPUT:  "[2026-03-04 14:29:55] @general CHAT [alice]:hello"
// OUTPUT: Message{Timestamp: ..., Type: MsgChat, Sender: "alice", Content: "hello", Room: "general"}

func ParseLogLine(line string) (Message, error) {
    // STEP 1: Parse timestamp from [YYYY-MM-DD HH:MM:SS]
    line = strings.TrimSpace(line)
    if line[0] != '[' {
        return Message{}, fmt.Errorf("invalid format: missing opening bracket")
    }
    closeBracket := strings.IndexByte(line, ']')
    tsStr := line[1:closeBracket]                    // "2026-03-04 14:29:55"
    // Sscanf extracts 6 integers from the timestamp string
    fmt.Sscanf(tsStr, "%d-%d-%d %d:%d:%d", &year, &month, &day, &hour, &min, &sec)
    ts := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.Local)

    // STEP 2: Extract optional @room tag
    rest := line[closeBracket+2:]                    // "@general CHAT [alice]:hello"
    var room string
    if rest[0] == '@' {
        spaceIdx := strings.IndexByte(rest, ' ')
        room = rest[1:spaceIdx]                      // "general"
        rest = rest[spaceIdx+1:]                     // "CHAT [alice]:hello"
    }

    // STEP 3: Match the type keyword and parse the payload
    if strings.HasPrefix(rest, "CHAT ") {
        inner := rest[5:]                            // "[alice]:hello"
        idx := strings.Index(inner, "]:")             // Find "]:" separator
        sender := inner[1:idx]                       // "alice"
        content := inner[idx+2:]                     // "hello"
        return Message{Timestamp: ts, Type: MsgChat, Sender: sender, Content: content, Room: room}, nil
    }
    // ... similar parsing for JOIN, LEAVE, NAMECHANGE, ANNOUNCE, MOD, SERVER
}
```

**Understanding the parsing step by step:**

```
Input: "[2026-03-04 14:29:55] @general CHAT [alice]:hello"

After STEP 1:  ts = 2026-03-04 14:29:55
               rest = "@general CHAT [alice]:hello"

After STEP 2:  room = "general"
               rest = "CHAT [alice]:hello"

After STEP 3:  type = MsgChat
               sender = "alice"
               content = "hello"
```

**Key Insight:** `FormatLogLine()` and `ParseLogLine()` are inverse functions. Any message formatted by one can be perfectly reconstructed by the other. This is verified by round-trip tests in `models/message_test.go`.

---

## What's Next

In the next lesson, you'll get **hands-on exercises** to practice navigating, tracing, and modifying this codebase.
