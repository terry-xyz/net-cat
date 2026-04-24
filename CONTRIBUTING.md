# Contributing to Net-Cat

Thank you for your interest in contributing to Net-Cat. This document covers the expected workflow, local development commands, and the main parts of the codebase you should understand before opening changes.

## Getting Started

1. Fork the repository on GitHub.
2. Clone your fork:

```bash
git clone https://github.com/YOUR_USERNAME/net-cat.git
cd net-cat
```

3. Create a feature branch:

```bash
git checkout -b feature/your-feature-name
```

## Development Requirements

- Go 1.25 or higher
- A terminal with standard TCP tooling for manual testing (`nc` or bash with `/dev/tcp`)
- No external Go dependencies are required for the project itself

## Code Standards

- Follow standard Go formatting and run `gofmt -w .` on touched files before committing
- Keep changes scoped and readable; avoid mixing unrelated refactors with feature work
- Add or update tests when behavior changes
- Update `README.md` or `CHANGELOG.md` when user-facing behavior or release notes change

## Testing

Run the full suite with:

```bash
go test ./...
```

Useful targeted commands:

```bash
go test -v ./...
go test ./server -run TestName
go test -cover ./...
```

## Building

Build the server binary with:

```bash
go build -o TCPChat
```

Run the server locally:

```bash
./TCPChat
```

## Submitting Changes

1. Ensure `go test ./...` passes.
2. Use clear commit messages following [Conventional Commits](https://www.conventionalcommits.org/):
   - `feat:` for new features
   - `fix:` for bug fixes
   - `docs:` for documentation updates
   - `test:` for test changes
   - `chore:` for maintenance work
3. Push your branch to your fork.
4. Open a Pull Request against `main`.

## Project Structure

These are the main areas contributors usually need to touch:

- `main.go`: entrypoint that validates CLI args, wires logging, starts the operator terminal, and launches the TCP server
- `server/`: core chat server logic including connection handling, rooms, moderation, shutdown, recovery, and operator commands
- `client/`: per-connection state, interactive terminal input handling, and socket write coordination
- `cmd/`: shared command registry and command parsing helpers
- `logger/`: daily log-file creation and append logic
- `models/`: shared message models and log formatting helpers
- `README.md`: user-facing setup and usage documentation
- `CHANGELOG.md`: release notes by version

## Code Review

All submissions go through review. A change is expected to meet these conditions:

- Tests pass locally
- Behavior matches the documented chat protocol and command semantics
- Concurrency-sensitive code is justified and covered by tests where practical
- Documentation is updated when commands, workflows, or operator behavior change

## Questions?

Open an issue in the [GitHub issue tracker](https://github.com/terry-xyz/net-cat/issues) if you want to discuss a bug, a feature, or a design change before opening a pull request.
