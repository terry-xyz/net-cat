# Exercises

These exercises are designed to take you from "I've never read Go before" to "I can modify this codebase confidently." Each level builds on the previous one — don't skip ahead until you can complete the current level without help.

**How to use these exercises:**
- Do them in order (the numbering matters)
- Write your answers on paper or in a text file — don't just think through them
- Check your answers against the code (`client/client.go`, `server/server.go`, etc.)
- If you get stuck, re-read the relevant teaching file (listed in each exercise)

---

## Level 0: Warmup (Absolute Beginners)

> **Skills you'll practice:** Basic file navigation, reading code structure, getting comfortable with Go syntax

### Exercise 0.1: Find a File
**Task:** List all the `.go` files in the `server/` folder. How many are there (excluding test files)?

**Hint:** Test files end with `_test.go`.

### Exercise 0.2: Read a Struct
**Task:** Open `client/client.go` and find the `Client` struct (starts around line 33). List all the fields that are exported (start with an uppercase letter). What's the difference between exported and unexported fields?

**Expected answer format:**
```
Exported fields: Conn, Username, JoinTime, IP
```

### Exercise 0.3: Count Functions
**Task:** Open `cmd/commands.go`. Count the number of functions defined in the file. List their names.

**Hint:** Functions start with `func`.

### Exercise 0.4: Understand a Comment
**Task:** Find a comment in `client/client.go` that explains **why** something is done (not just **what**). Write it down and explain in your own words why that design choice was made.

**Hint:** Look for comments above the `mu` field or the `writeLoop` function.

### Exercise 0.5: Trace an Import
**Task:** Open `server/handler.go`. Find two imports: one from the Go standard library (like `"fmt"` or `"net"`) and one from this project (like `"net-cat/client"`). What's the difference between these two types of imports?

---

## Level 1: Find (Read the Code)

> **Skills you'll practice:** Locating specific code, understanding definitions, building a mental map

### Exercise 1.1: Find the Capacity Limit
**Task:** What is the maximum number of clients that can be actively chatting at the same time? Find the exact constant and its file:line location.

### Exercise 1.2: Find All Message Types
**Task:** Open `models/message.go`. List every `MessageType` constant (there are 7). For each one, write a one-sentence description of what event it represents.

### Exercise 1.3: Find the Command Registry
**Task:** How many commands does the server support? List them all with their minimum privilege level (User, Admin, or Operator). Which commands can a regular user run?

### Exercise 1.4: Find the Validation Rules
**Task:** Open `server/handler.go` and find the `ValidateName()` function. List every rule that a username must satisfy to be accepted. What is the maximum length?

### Exercise 1.5: Find the Reserved Names
**Task:** Which name(s) are reserved and cannot be used by any client? Where is this defined?

### Exercise 1.6: Find the Banner
**Task:** What ASCII art does a new client see when they connect? Find the constant that holds it. What is the name prompt that follows it?

---

## Level 2: Trace (Follow the Data)

> **Skills you'll practice:** Data flow, mental execution, predicting behavior. **This is the MOST important skill.** If you can trace data through the system, you understand the codebase.

### Exercise 2.1: Trace a Chat Message
**Task:** Alice types "hello" and presses Enter. Trace the exact path of this message through the code. List every function that touches it, in order, with file:line references.

**Expected format:**
```
1. ReadLineInteractive() reads "hello" (client/client.go:328)
2. ParseCommand("hello") returns false (cmd/commands.go:47)
3. handleChatMessage() is called (server/handler.go:336)
4. ...
```

### Exercise 2.2: Trace a Join Event
**Task:** Bob connects and types "Bob" as his name. What messages do the OTHER connected clients see? Trace the code path that produces these messages.

### Exercise 2.3: Trace a Kick
**Task:** Admin Alice types `/kick Bob`. List every side effect that happens (messages sent, map modifications, log entries, queue changes). Don't miss any.

**Hint:** There are at least 7 distinct side effects. Check `cmdKick()` in `handler.go`.

### Exercise 2.4: Predict the Output
**Task:** The server has 10 active clients. An 11th person connects. What exactly do they see on their terminal? Trace through `checkOrQueue()` and predict the text output word-for-word.

### Exercise 2.5: Trace a Shutdown
**Task:** The operator presses Ctrl+C. In what order do these things happen?
1. The `quit` channel is closed
2. Goodbye messages are sent
3. The listener is closed
4. The logger is closed
5. `shutdownDone` is closed

Write the exact order and explain what each step unblocks.

### Exercise 2.6: Mental Execution — History Recovery
**Task:** The server crashed and restarts. Today's log file contains:
```
[2026-02-24 14:00:00] JOIN Alice
[2026-02-24 14:00:05] CHAT [Alice]:hello
[2026-02-24 14:00:10] SERVER Server started on port 8989
[2026-02-24 14:01:00] LEAVE Alice voluntary
```
After `RecoverHistory()` runs, how many messages are in the in-memory history? Which ones, and why?

**Hint:** Not all log line types are added to history. Check the filter in `RecoverHistory()`.

---

## Level 3: Modify (Small Changes)

> **Skills you'll practice:** Targeted changes, understanding ripple effects, testing your modifications

### Exercise 3.1: Change the Max Message Length
**Task:** Currently, chat messages are limited to 2048 characters. Change this to 1024 characters.
- Which file(s) need to change?
- How many places reference this limit?
- Would you also need to change any tests?

**Don't actually change the code** — just identify the locations and plan the change.

### Exercise 3.2: Add a New Reserved Name
**Task:** You want to prevent anyone from using the name "Admin" (in addition to "Server"). Where would you add this? Write the exact code change.

### Exercise 3.3: Change the Kick Cooldown
**Task:** Currently, kicked users are blocked for 5 minutes. Change this to 10 minutes.
- Find the exact line that sets the cooldown duration.
- What would happen if you changed only that line? Are there any other places that reference this duration?

### Exercise 3.4: Add a New Message to Banned Users
**Task:** Currently, when a banned user tries to reconnect, they see: `"You are banned from this server.\n"`. Change this to include a hint: `"You are banned from this server. Contact the administrator for appeal.\n"`
- Find the exact line to change.
- Are there other places that display the same or similar message?

### Exercise 3.5: Modify the Welcome Banner
**Task:** Add a line saying `"Type /help for available commands.\n"` right after the penguin ASCII art but before the name prompt.
- Where is the banner defined?
- How would you modify it without breaking the name prompt flow?
- Would any tests break?

---

## Level 4: Extend (Add Features)

> **Skills you'll practice:** Planning additions, maintaining consistency with existing patterns, considering edge cases

### Exercise 4.1: Add a `/uptime` Command
**Task:** Design a new `/uptime` command that tells the user how long the server has been running.

Plan your implementation:
1. What privilege level should it have?
2. Where do you register the command? (Which file, which map?)
3. Where do you store the server start time?
4. Where do you implement the command handler?
5. What format should the output be?

Write pseudo-code for the handler function.

### Exercise 4.2: Add a `/who` Command
**Task:** Design a `/who <name>` command that shows information about a specific user: their join time and idle duration.

Consider:
1. What happens if the user doesn't exist?
2. What happens if no argument is given?
3. Should this command be user-level or admin-level?
4. What existing command handler can you use as a template?

### Exercise 4.3: Add Message Rate Limiting
**Task:** Design rate limiting so no user can send more than 5 messages per 10 seconds.

Think about:
1. Where do you store the rate limit state? (Which struct? Which field?)
2. Where do you check the rate limit? (Which function?)
3. What message does the user see when rate-limited?
4. How do you handle the timing? (Sliding window? Token bucket? Simple counter?)
5. Does the rate limit need mutex protection? Why or why not?

### Exercise 4.4: Add a `/stats` Command (Operator Only)
**Task:** Design a `/stats` command for the operator terminal that shows:
- Number of connected clients
- Number of queued clients
- Total messages sent today (from history length)
- Server uptime

Plan which existing functions you'd call to gather this data. Write the implementation skeleton.

---

## Level 5: Break & Fix (Debugging)

> **Skills you'll practice:** Understanding WHY code exists, defensive reasoning, debugging without running code

### Exercise 5.1: What If We Remove `closeOnce`?
**Task:** In `client/client.go`, the `Close()` method uses `sync.Once`:
```go
func (c *Client) Close() {
    c.closeOnce.Do(func() {
        close(c.done)
        c.Conn.Close()
    })
}
```
What would happen if you removed the `sync.Once` and just called `close(c.done)` directly? Describe the specific error and when it would occur.

**Hint:** Think about what happens when multiple goroutines call `Close()` on the same client.

### Exercise 5.2: What If `enqueue` Used a Blocking Send?
**Task:** The `enqueue` function drops messages if the channel is full:
```go
select {
case c.msgChan <- m:
case <-c.done:
default:   // DROP if full
}
```
What would happen if you removed the `default` case (making the send blocking)? Describe the failure scenario step by step.

### Exercise 5.3: What If We Removed the `errCapacityFull` Re-queue?
**Task:** In `handleConnection()`, there's a check after `RegisterClient()`:
```go
if err == errCapacityFull {
    c.Send("Chat became full. Entering queue...\n")
    if !s.checkOrQueue(c) { ... }
}
```
Under what scenario would `RegisterClient()` return `errCapacityFull` even though `checkOrQueue()` passed earlier? (The answer involves concurrency.)

### Exercise 5.4: Find the Race Condition
**Task:** Imagine we changed `RegisterClient()` to use `RLock` for the capacity check and `Lock` only for the insertion:
```go
func (s *Server) RegisterClient(c *client.Client, name string) error {
    s.mu.RLock()
    if len(s.clients) >= MaxActiveClients {
        s.mu.RUnlock()
        return errCapacityFull
    }
    s.mu.RUnlock()
    // ← What could go wrong between these two locks?
    s.mu.Lock()
    s.clients[name] = c
    s.mu.Unlock()
    return nil
}
```
Describe the exact race condition. What invariant could be violated?

### Exercise 5.5: The Midnight Bug
**Task:** The midnight watcher clears history at midnight:
```go
func (s *Server) startMidnightWatcher() {
    for {
        now := time.Now()
        nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
        // ...
        case <-timer.C:
            s.ClearHistory()
    }
}
```
What would happen if this function also closed and reopened the logger's file? Why does it NOT do this? (Hint: look at how `logger.ensureFile()` works.)

### Exercise 5.6: Explain the Double-Select in `enqueue()`
**Task:** Why does `enqueue()` have TWO `select` statements instead of one combined one?

```go
func (c *Client) enqueue(m writeMsg) {
    select {           // First select
    case <-c.done:
        return
    default:
    }
    select {           // Second select
    case c.msgChan <- m:
    case <-c.done:
    default:
    }
}
```

Could you merge them into one `select` with three cases? If so, why didn't the author? If not, what would break?

---

## The Vibecoding Graduation Test

Can you do these **WITHOUT AI assistance**?

### 1. Pseudo-code First
Write pseudo-code for adding a `/broadcast-admin` command that sends a message only to admin users.

### 2. Predict Files
Which files would you need to modify to add a new message type called `MsgWhisper` that gets logged and can be recovered from the log file?

### 3. Explain the Why
Why does the heartbeat probe write `\x00` instead of using TCP keepalive alone? (Hint: TCP keepalive is OS-level — the application can't set custom timeouts on all platforms.)

### 4. Find the Edge Case
What happens if a client sends a message at 23:59:59, the midnight watcher fires at 00:00:00, and a new client connects at 00:00:01? Will the new client see the 23:59:59 message in their history? Trace through the code to verify.

### 5. Rubber Duck
Explain the entire lifecycle of a client (connect → name → chat → disconnect) out loud, including every goroutine that gets created and destroyed, without looking at the code.

---

**If you can do all five, you've graduated from vibecoding to understanding.**

---

## What to Read Next

Learn about the tricky spots where bugs love to hide: [06 - Gotchas](06-gotchas.md)
