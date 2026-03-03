# Lesson 05: Exercises

---

## Level 0: Warmup (Absolute Beginners)

> **Skills:** Basic file navigation, reading code structure

### Exercise 0.1: Find a File
**Task:** List all the `.go` files in the `server/` directory. How many are there (excluding test files)?

### Exercise 0.2: Read a Struct
**Task:** Find the `Client` struct in `client/client.go`. List all its exported fields (the ones that start with a capital letter).

### Exercise 0.3: Count Functions
**Task:** Count how many functions are defined in `models/message.go`. List their names.

### Exercise 0.4: Understand a Comment
**Task:** Find the comment above the `Message` struct in `models/message.go`. It explains what each field means for each message type. For `MsgModeration`, what does `Sender` represent? What does `Extra` represent?

### Exercise 0.5: Trace an Import
**Task:** Open `server/handler.go`. Find one import from the Go standard library (like `"fmt"` or `"time"`) and one import from this project (like `"net-cat/client"`). What's the difference in how they're written?

---

## Level 1: Find (Read the Code)

> **Skills:** Locating specific code, understanding definitions

### Exercise 1.1: The Command Registry
**Task:** Open `cmd/commands.go`. How many commands are registered? Which commands require `PrivAdmin`? Which require `PrivOperator`?

<details>
<summary>Hint</summary>
Look at the `Commands` map starting at line 23. Count the entries. Check the `MinPriv` field.
</details>

### Exercise 1.2: The Welcome Banner
**Task:** Find the ASCII art that users see when they connect. Which file and line is it defined at?

<details>
<summary>Hint</summary>
Search for `WelcomeBanner` in the `server/` directory.
</details>

### Exercise 1.3: Reserved Names
**Task:** What names are reserved and cannot be used as usernames? Where is this list defined?

<details>
<summary>Hint</summary>
Look for `reservedNames` in `server/server.go`.
</details>

### Exercise 1.4: Message Length Limit
**Task:** What's the maximum length for a chat message? Where is this limit enforced?

<details>
<summary>Hint</summary>
Search for `2048` in `server/commands.go`.
</details>

### Exercise 1.5: Kick Cooldown Duration
**Task:** How long is a kicked user blocked from reconnecting? Where is this duration defined?

<details>
<summary>Hint</summary>
Look in `server/moderation.go` for `AddKickCooldown`.
</details>

---

## Level 2: Trace (Follow the Data)

> **Skills:** Data flow, mental execution, predicting behavior
>
> This is the MOST important skill. Being able to trace data through a system is what separates "I can read code" from "I understand code."

### Exercise 2.1: Trace a Chat Message
**Task:** Starting from `handleChatMessage` in `server/commands.go:14`, trace what happens when user "alice" sends "hello" in room "general." Write down each function called and what it does.

<details>
<summary>Hint</summary>
Follow: handleChatMessage → recordRoomEvent → AddRoomHistory + Logger.Log → BroadcastRoom → each client's Send → writeLoop
</details>

### Exercise 2.2: Trace a Name Change
**Task:** What happens when a user types `/name newname`? Trace through `cmdName` in `server/commands.go:264`. What functions are called? What gets broadcast? What file on disk might change?

<details>
<summary>Hint</summary>
The admin rename (`RenameAdmin`) triggers `SaveAdmins()` which writes to `admins.json`.
</details>

### Exercise 2.3: Trace a Disconnect
**Task:** A user's network drops while they're in room "lobby." Their `ReadLineInteractive` returns an error. Trace what happens next. What messages do other users see? What happens to the room if it's now empty?

<details>
<summary>Hint</summary>
Error → SetDisconnectReason("drop") → return → defer runs → RemoveClient → recordRoomEvent(MsgLeave) → BroadcastRoom → admitFromRoomQueue → deleteRoomIfEmpty
</details>

### Exercise 2.4: Trace a Ban
**Task:** Admin "bob" types `/ban alice`. Alice is in room "general" and there's another user "alice2" connecting from the same IP in room "lobby." What happens to both users? What happens to someone queued from the same IP?

<details>
<summary>Hint</summary>
Read `cmdBan` in `server/commands.go:398-464`. Note the `GetClientsByIP` call and the queue cleanup.
</details>

### Exercise 2.5: Predict the Output
**Task:** The server has been running. The log file contains:
```
[2026-03-04 10:00:00] @general JOIN alice
[2026-03-04 10:00:05] @general CHAT [alice]:hello
[2026-03-04 10:00:10] @general JOIN bob
```
The server crashes and restarts. A user "charlie" connects and joins "general." What messages does charlie see before getting their prompt?

<details>
<summary>Hint</summary>
RecoverHistory reads the log, skips SERVER events, and adds to room history. Display() renders each message. Charlie would see the join/chat/join messages in order.
</details>

---

## Level 3: Modify (Small Changes)

> **Skills:** Targeted changes, ripple effects, testing

### Exercise 3.1: Change the Default Port
**Task:** Change the default port from `8989` to `9090`. Which file do you modify? How do you verify it works?

<details>
<summary>Answer</summary>
Change line 12 of `main.go`: `port := "9090"`. Run `go build && ./net-cat` and verify the output says "Listening on the port :9090".
</details>

### Exercise 3.2: Change the Max Room Capacity
**Task:** Change the maximum users per room from 10 to 20. Which constant do you change? What test(s) might be affected?

<details>
<summary>Answer</summary>
Change `MaxActiveClients` in `server/server.go:172`. Check `server/integration_test.go` for tests that depend on this value (they fill rooms to capacity).
</details>

### Exercise 3.3: Add a New Reserved Name
**Task:** Add "Admin" as a reserved name (in addition to "Server"). Which file and map do you modify?

<details>
<summary>Answer</summary>
In `server/server.go:61-63`, add `"Admin": true` to the `reservedNames` map.
</details>

### Exercise 3.4: Increase Kick Cooldown
**Task:** Change the kick cooldown from 5 minutes to 15 minutes. Which file and line?

<details>
<summary>Answer</summary>
`server/moderation.go:24`: change `5 * time.Minute` to `15 * time.Minute`.
</details>

### Exercise 3.5: Custom Disconnect Message
**Task:** When a user types `/quit`, they currently just disconnect silently (the handler returns). Modify the code so the user sees "Goodbye!" before disconnecting.

<details>
<summary>Hint</summary>
In `server/commands.go:60-61`, before `return true`, add `c.Send("Goodbye!\n")`.
</details>

---

## Level 4: Extend (Add Features)

> **Skills:** Planning additions, maintaining consistency

### Exercise 4.1: Add a `/who` Command
**Task:** Design a new `/who` command that shows all users across ALL rooms (not just the current room). Plan which files you'd modify:
1. Where do you register the command?
2. Where do you implement the handler?
3. Where do you add the dispatch case?
4. What privilege level should it have?

<details>
<summary>Plan</summary>

1. `cmd/commands.go` — add to `Commands` map and `CommandOrder`
2. `server/commands.go` — add `cmdWho` method
3. `server/commands.go:59-91` — add `case "who":` in `dispatchCommand`
4. `PrivUser` — any user should see who's online

The implementation would iterate `s.GetRoomNames()` and `s.GetRoomClientNames(room)`.
</details>

### Exercise 4.2: Add Message Timestamps to History Display
**Task:** Currently, when a user joins a room, they see the full history with timestamps. Design a feature to show only the last N messages (e.g., last 50). Where would you add the limit? How would you modify `GetRoomHistory`?

### Exercise 4.3: Add a `/me` Action Command
**Task:** Design a `/me` command that sends action messages like `* alice waves`. Think about:
- Do you need a new `MessageType`?
- How would it display vs. a regular chat message?
- Should it be logged differently?

### Exercise 4.4: Room Passwords
**Task:** Design a feature where room creators can set a password. Think through:
- Where would you store the password? (What field on `Room`?)
- When would you prompt for it? (During `/switch` or `/create`?)
- Should the password be logged?
- What happens when the last user leaves a password-protected room?

---

## Level 5: Break & Fix (Debugging)

> **Skills:** Understanding WHY code exists, debugging

### Exercise 5.1: Remove the Mutex
**Task:** Imagine you removed `s.mu.Lock()` from `RegisterClient`. What bug would this cause? Describe a specific scenario with two users connecting simultaneously.

<details>
<summary>Answer</summary>
Both goroutines check `s.clients["alice"]` and see it's empty. Both proceed to insert. The second insert overwrites the first, leaving the first connection orphaned — they think they're registered, but the server doesn't know about them.
</details>

### Exercise 5.2: Remove the `closeOnce`
**Task:** What happens if you replace `c.closeOnce.Do(func() { ... })` with just `close(c.done); c.Conn.Close()`? When would this crash?

<details>
<summary>Answer</summary>
If the heartbeat detects a dead connection at the same time the handler reads an error, both call `c.Close()`. The second `close(c.done)` panics: "close of closed channel."
</details>

### Exercise 5.3: Remove the `skipLF` Flag
**Task:** What happens if you delete the `skipLF` logic from `ReadLineInteractive`? What would Windows netcat users experience?

<details>
<summary>Answer</summary>
Windows sends `\r\n` for Enter. Without `skipLF`, the server would process `\r` (returning the line) and then `\n` (returning an empty line). Every message would be followed by a blank message.
</details>

### Exercise 5.4: Break the Atomic Write
**Task:** In `SaveAdmins`, what if you wrote directly to `admins.json` instead of the temp file + rename? Describe a scenario where this causes data loss.

<details>
<summary>Answer</summary>
1. Server starts writing to `admins.json` (truncates the file first with `WriteFile`)
2. Server crashes mid-write
3. On restart, `LoadAdmins` reads a half-written JSON file → parse error → all admins lost
</details>

### Exercise 5.5: The Non-Blocking Send
**Task:** In `client.go:93-105`, the `enqueue` function drops messages when the channel is full. Why is this the correct behavior? What would happen if it blocked instead?

<details>
<summary>Answer</summary>
If `enqueue` blocked, a single slow client could stall the broadcast. The server goroutine holding `s.mu` would block on `c.msgChan <-`, preventing ALL other clients from sending or receiving. The entire server would freeze until that one client's buffer drains.
</details>

---

## The Vibecoding Graduation Test

Can you do these **WITHOUT AI assistance?**

1. **Pseudo-code first:** Write pseudo-code for a `/clear` command that clears the current room's chat history for all users.

2. **Predict files:** Which files would adding a `/clear` command touch?

3. **Explain the why:** Why does `writeWithContinuity` assemble the entire output into a single buffer before calling `Conn.Write`, instead of making multiple write calls?

4. **Find the edge case:** What happens if a user sends a message that is exactly 2048 characters long? What about 2049?

5. **Rubber duck:** Explain the heartbeat system out loud — pretend you're explaining it to someone who has never written Go. Cover: why it exists, what it sends, how it detects dead clients, and why it uses a goroutine for the probe.

If you can do all five, you've graduated from vibecoding to understanding.

---

## What's Next

In the next lesson, you'll learn about **gotchas** — the subtle bugs and tricky behaviors that are easy to miss.
