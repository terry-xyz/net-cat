# line-by-line: main.go

## File Purpose

Process bootstrap. This file validates CLI input, wires the logger and server together, installs interrupt handling, starts the operator loop, and transfers control into the TCP server.

## Why This File Matters

If startup semantics change, everything downstream changes. It also defines two key operational guarantees: invalid ports fail fast, and Ctrl+C triggers graceful shutdown rather than abrupt process exit.

## Dependencies In and Out

- Outbound: `logger.New`, `server.New`, `srv.Start`, `srv.Shutdown`, `srv.StartOperator`.
- Inbound: no internal callers; this is the process entrypoint.

## Ordered Walkthrough

- `main.go:1-9`: package declaration and imports. `fmt` and `os` handle CLI and diagnostic output, `os/signal` handles interrupts, and `logger` plus `server` are the two internal subsystems wired here.
- `main.go:11-23`: `main()` starts with default port `8989`, rejects more than one positional argument, optionally overrides the port from `os.Args[1]`, and rejects invalid ports using `isValidPort`. The repeated usage print is intentional: all invalid CLI shapes collapse to the same message.
- `main.go:25`: `server.New(port)` constructs the in-memory server object before any network side effects happen.
- `main.go:27-31`: logger creation is best-effort. Failure prints a warning to stderr but does not abort startup. That is why server code must tolerate `srv.Logger == nil`.
- `main.go:33-42`: the interrupt goroutine consumes the first `os.Interrupt`, calls `srv.Shutdown()`, then keeps draining later interrupts so the process is not killed by the default signal handler before graceful shutdown finishes.
- `main.go:44-45`: the operator terminal starts in a separate goroutine and reads commands from `stdin`. This creates a second control plane separate from TCP clients.
- `main.go:47-50`: `srv.Start()` owns the main lifecycle after setup. Any returned error is fatal and printed to stderr.
- `main.go:53-69`: `isValidPort` avoids `strconv` and parses digits manually. It rejects empty strings, non-digits, values above `65535`, and `0`. Leading zeros are allowed because the parsed integer is what matters.

## Key Takeaways

- Startup is intentionally resilient to logger failure.
- Signal handling is structured to preserve graceful shutdown.
- Port parsing is simple and explicit, with no whitespace tolerance.

## Audit Notes

- If you add more CLI flags, this file becomes the first place where argument shape and backward compatibility can break.
- If you make logger initialization mandatory, you must revisit tests and `server/history.go`, which currently assume logging can be absent.
