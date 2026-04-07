# line-by-line: client/client.go

## File Purpose

Connection wrapper for one TCP client. It centralizes output serialization, close semantics, heartbeat metadata, privilege flags, and interactive character-by-character input handling.

## Why This File Matters

This file is the transport safety boundary. If it behaves incorrectly, prompts get corrupted, writes interleave, backpressure spreads across clients, or disconnect handling becomes racy.

## Dependencies In and Out

- Inbound: used by `server/handler.go`, `server/server.go`, `server/commands.go`, and `server/operator.go`.
- Outbound: wraps `net.Conn`, `bufio.Scanner`, and time-based bookkeeping.

## Ordered Walkthrough

- `client/client.go:11-17`: constants define hard limits. `maxLineLength` caps scanner input during non-interactive reads. `msgChanSize` is intentionally large to keep one slow client from backpressuring the rest of the system. `maxInteractiveBuf` bounds redraw state.
- `client/client.go:18-26`: write message types encode the write loop protocol. The distinction between `wmMessage`, `wmPrompt`, `wmEcho`, `wmBackspace`, and `wmNewline` is what lets asynchronous notifications coexist with interactive typing.
- `client/client.go:27-32`: `writeMsg` is the queue payload written into `msgChan`.
- `client/client.go:33-67`: `Client` stores connection identity plus concurrency state. The main design split is: some fields are guarded by `mu`, some are write-loop-only (`inputBuf`, `prompt`), and `skipLF` is handler-only.
- `client/client.go:68-82`: `NewClient` captures the remote address, creates buffered channels, installs a scanner with a larger buffer than the default, and starts the dedicated write goroutine immediately.
- `client/client.go:83-92`: `Send` and `SendPrompt` both enqueue work instead of touching the socket directly.
- `client/client.go:93-108`: `enqueue` first checks whether the client is already done, then attempts a non-blocking send into `msgChan`. Dropping when full is a deliberate isolation choice: a stuck client should lose messages rather than stall the server.
- `client/client.go:109-119`: `ReadLine` uses the scanner for onboarding-style full-line reads. EOF and scanner errors are translated into the usual `(string, error)` pattern.
- `client/client.go:120-144`: `Close`, `IsClosed`, and `Done` define idempotent close behavior around `closeOnce` and the `done` channel.
- `client/client.go:145-151`: `ResetScanner` rebuilds the scanner after queue admission cancels a read with a deadline. Without this, the scanner could stay poisoned.
- `client/client.go:152-235`: getter and setter methods wrap mutable cross-goroutine fields: heartbeat timestamps, disconnect reason, mute bit, admin bit, and last activity.
- `client/client.go:236-281`: `writeLoop` is the single writer. Before echo mode, messages are written raw. After echo mode, the loop interprets message types and updates prompt/input tracking. The most important branch is `wmMessage`, which delegates to redraw-aware output.
- `client/client.go:282-315`: `writeWithContinuity` builds one output buffer containing clear-line escape sequence, incoming message, prompt, and partial input. One batched write avoids partial-write deadlocks in pipe-backed tests.
- `client/client.go:316-325`: `SetEchoMode` flips the client into interactive redraw mode after onboarding completes.
- `client/client.go:326-374`: `ReadLineInteractive` reads one byte at a time, handles CRLF normalization, backspace, null-byte heartbeat probes, printable ASCII echo, and newline submission. It never writes directly; it only enqueues write-loop actions.

## Key Takeaways

- The file splits input into two phases: scanner-based onboarding and byte-based interactive chat.
- The write goroutine is the core transport invariant.
- Prompt continuity depends on the queue protocol, not on ad hoc socket writes.

## Audit Notes

- If you change `msgChanSize`, `enqueue`, or `writeLoop`, inspect all input-continuity and backpressure tests.
- If you expand accepted character sets beyond printable ASCII in `ReadLineInteractive`, revisit redraw, length, and control-character assumptions.
