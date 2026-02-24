# Net-Cat: Project Overview

## Your Goal

You used AI to help build (or explore) this project. That's a great starting point — but "vibecoding" (accepting AI-generated code without understanding it) creates a fragile foundation. These lessons will take you from "it works and I don't know why" to "I understand every moving part and can change anything myself."

By the end you'll be able to:
- Read any file in this project and explain what it does
- Trace a message from the moment someone types it to the moment everyone sees it
- Add new commands or features without breaking existing ones
- Debug problems by reasoning about the code, not by asking AI

---

## What is Code? (If You're New)

Think of code like a **recipe**. A recipe has **ingredients** (data) and **instructions** (logic). A program is the same — data goes in, instructions transform it, and results come out.

Every program is built from just 5 building blocks:

| Building Block | Plain English | Go Example |
|----------------|---------------|------------|
| **Variable** | A labeled box that holds a value | `name := "Alice"` |
| **Function** | A reusable recipe with a name | `func add(a, b int) int` |
| **Condition** | A yes/no question | `if x > 10 { ... }` |
| **Loop** | Repeat until done | `for i := 0; i < 10; i++` |
| **Data Structure** | An organized container | `map[string]*Client` (a dictionary) |

That's it. Every program — including this one — is just these five things combined in different ways.

---

## What This Project Does (One Sentence)

**Net-Cat is a terminal-based group chat server** — you run it, people connect with `nc` or `telnet`, and everyone can talk to each other in real time through their terminal.

Think of it like a group text chat, but instead of a phone app, you use a plain terminal window.

---

## High-Level Architecture

```
                    ┌──────────────────────────────────────────┐
                    │               main.go                    │
                    │  Parses port, wires logger, starts server│
                    └────────────────┬─────────────────────────┘
                                     │
                    ┌────────────────▼─────────────────────────┐
                    │           server/server.go               │
                    │  TCP listener, client map, history,      │
                    │  queue, moderation, admin persistence,   │
                    │  heartbeat, broadcast, operator terminal │
                    └────────────────┬─────────────────────────┘
                                     │
                    ┌────────────────▼─────────────────────────┐
                    │           server/handler.go              │
                    │  Per-connection lifecycle:                │
                    │  welcome → name → history → message loop │
                    │  Command dispatch (/kick, /ban, etc.)    │
                    └──────┬──────────────────┬────────────────┘
                           │                  │
              ┌────────────▼──────┐   ┌───────▼───────────────┐
              │  client/client.go │   │   cmd/commands.go     │
              │  Connection wrap, │   │   Command registry,   │
              │  write goroutine, │   │   parsing, privilege  │
              │  echo mode, I/O   │   │   levels              │
              └────────────────── ┘   └───────────────────────┘
                           │
              ┌────────────▼──────┐   ┌───────────────────────┐
              │ models/message.go │   │   logger/logger.go    │
              │  Message types,   │   │   Daily log files,    │
              │  formatting,      │   │   thread-safe writes  │
              │  log parsing      │   │                       │
              └───────────────────┘   └───────────────────────┘
```

---

## What Each Folder Does

| Folder | Plain English | Key File |
|--------|--------------|----------|
| **(root)** | Entry point. Parses the port number and starts everything. | `main.go` |
| **server/** | The brain. Manages all connections, routing messages, moderation, and the queue. | `server.go`, `handler.go` |
| **client/** | One client = one connected person. Handles reading input and writing output for a single connection. | `client.go` |
| **cmd/** | The command registry. Defines every `/command` — its name, who can use it, and its description. | `commands.go` |
| **models/** | Data shapes. Defines what a "message" looks like and how to format it for display or logging. | `message.go` |
| **logger/** | The diary. Writes everything that happens to daily log files so history survives server restarts. | `logger.go` |
| **logs/** | (Auto-created) Where daily log files land. Not committed to git. | `chat_YYYY-MM-DD.log` |

---

## Entry Point: Where the Program Starts

Every Go program starts at the `main()` function in `main.go`. Here's what it does, step by step:

1. **Reads the port** from command-line arguments (default: `8989`)
2. **Creates a server** (`server.New(port)`)
3. **Creates a logger** that writes to the `logs/` folder
4. **Sets up Ctrl+C handling** so the server shuts down cleanly
5. **Starts the operator terminal** (reads commands from the keyboard)
6. **Starts the server** (begins accepting connections — blocks here until shutdown)

---

## Exit Points: How the Program Ends

The program ends when:
- The operator presses **Ctrl+C** → triggers `srv.Shutdown()`
- Shutdown sends goodbye messages, waits up to 5 seconds, then force-closes everything
- The logger flushes, and the process exits

---

## Files at a Glance

```
net-cat/
├── main.go                  ← START HERE: entry point
├── go.mod                   ← Go module declaration (no dependencies)
├── CHANGELOG.md             ← Version history
├── README.md                ← User guide
│
├── server/
│   ├── server.go            ← Server struct, Start/Shutdown, broadcast, queue, moderation
│   └── handler.go           ← Per-connection lifecycle, all /command implementations
│
├── client/
│   └── client.go            ← Client struct, write goroutine, echo mode input
│
├── cmd/
│   └── commands.go          ← Command definitions and parsing
│
├── models/
│   └── message.go           ← Message struct, formatting for display and logs
│
├── logger/
│   └── logger.go            ← Thread-safe daily log file writer
│
└── teachings/               ← You are here
```

---

## What to Read Next

| Lesson | What You'll Learn |
|--------|-------------------|
| [01 - Core Concepts](01-core-concepts.md) | Go syntax basics, goroutines, channels, mutexes |
| [02 - Data Flow](02-data-flow.md) | How a message travels from typing to everyone's screen |
| [03 - Patterns](03-patterns.md) | Recurring design patterns used throughout the code |
| [04 - Line by Line](04-line-by-line.md) | Detailed walkthrough of key functions |
| [05 - Exercises](05-exercises.md) | Hands-on tasks from beginner to advanced |
| [06 - Gotchas](06-gotchas.md) | Tricky spots where bugs love to hide |
| [07 - Glossary](07-glossary.md) | Every term and abbreviation decoded |
